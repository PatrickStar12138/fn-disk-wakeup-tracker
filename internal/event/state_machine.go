package event

import (
	"sync"
	"time"
)

// Observation 表示 Collector 的单次内存样本，不会直接写入数据库。
type Observation struct {
	// Disk 是本轮扫描识别的物理盘元数据和候选状态。
	Disk Disk
	// At 是带单调分量的本轮采样时间。
	At time.Time
	// ReadIO 和 WriteIO 是内核累计字节计数。
	ReadIO  uint64
	WriteIO uint64
	// Evidence 是同一有限时间窗口内的疑似进程证据。
	Evidence []Evidence
}

// samplePoint 是每块磁盘最多保留 64 个的高频内存环形样本。
type samplePoint struct {
	// At 是样本时间。
	At time.Time
	// ReadIO 和 WriteIO 是累计字节计数。
	ReadIO, WriteIO uint64
	// State 是当次候选状态。
	State DiskState
}

// diskTrack 保存单块磁盘的内存防抖状态和 I/O 计数基线。
type diskTrack struct {
	disk         Disk
	present      bool
	pending      DiskState
	pendingCount int
	lastRead     uint64
	lastWrite    uint64
	// history 是有界内存环形缓冲区，绝不逐样本持久化到 SQLite。
	history                   [64]samplePoint
	historyNext, historyCount int
}

// StateMachine 串行处理多盘状态变化，避免重复和虚假唤醒事件。
type StateMachine struct {
	mu            sync.Mutex
	confirmations int
	disks         map[string]*diskTrack
	collectorUp   bool
}

// NewStateMachine 创建状态机，并把非法确认次数收敛为一次。
func NewStateMachine(confirmations int) *StateMachine {
	if confirmations < 1 {
		confirmations = 1
	}
	return &StateMachine{confirmations: confirmations, disks: make(map[string]*diskTrack), collectorUp: true}
}

// Seed 从数据库恢复当前状态，不产生 Collector 或 Server 重启后的虚假唤醒事件。
func (m *StateMachine) Seed(disks []Disk) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range disks {
		m.disks[d.ID] = &diskTrack{disk: d, present: d.Present}
	}
}

// UpdateConfirmations 在现有状态机内安全更新待机确认次数，不创建重复采集循环。
func (m *StateMachine) UpdateConfirmations(confirmations int) {
	if confirmations < 1 {
		confirmations = 1
	}
	m.mu.Lock()
	m.confirmations = confirmations
	m.mu.Unlock()
}

// Observe 处理一个磁盘样本；高频数据只更新内存，仅返回需要持久化的状态事件。
func (m *StateMachine) Observe(o Observation) []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o.At.IsZero() {
		o.At = time.Now()
	}
	t := m.disks[o.Disk.ID]
	if t == nil {
		o.Disk.PreviousState = StateUnknown
		o.Disk.LastStateChange = o.At
		t = &diskTrack{disk: o.Disk, present: o.Disk.Present, lastRead: o.ReadIO, lastWrite: o.WriteIO}
		t.history[0], t.historyNext, t.historyCount = samplePoint{At: o.At, ReadIO: o.ReadIO, WriteIO: o.WriteIO, State: o.Disk.State}, 1, 1
		m.disks[o.Disk.ID] = t
		return nil
	}

	readDelta := saturatingDelta(o.ReadIO, t.lastRead)
	writeDelta := saturatingDelta(o.WriteIO, t.lastWrite)
	t.lastRead, t.lastWrite = o.ReadIO, o.WriteIO
	t.history[t.historyNext] = samplePoint{At: o.At, ReadIO: o.ReadIO, WriteIO: o.WriteIO, State: o.Disk.State}
	t.historyNext = (t.historyNext + 1) % len(t.history)
	if t.historyCount < len(t.history) {
		t.historyCount++
	}
	if readDelta > 0 || writeDelta > 0 {
		o.Disk.LastActivity = o.At
	}

	if !o.Disk.Present {
		if t.present {
			t.present = false
			return []Record{newRecord(t.disk, StateUnknownEvent, t.disk.State, StateUnknown, o.At, readDelta, writeDelta, nil, "设备暂时离线或已拔出", "高")}
		}
		return nil
	}
	if !t.present {
		t.present = true
		t.disk = o.Disk
		t.disk.PreviousState = StateUnknown
		t.disk.LastStateChange = o.At
		t.pendingCount = 0
		// Reattach is recorded as activity, never as an ordinary wakeup.
		return []Record{newRecord(t.disk, DiskActivity, StateUnknown, o.Disk.State, o.At, readDelta, writeDelta, o.Evidence, "设备重新接入，已抑制普通唤醒判断", "高")}
	}

	if o.Disk.State == t.disk.State {
		t.pending, t.pendingCount = "", 0
		t.disk = mergeDisk(t.disk, o.Disk, false, o.At)
		return nil
	}

	needed := 1
	if o.Disk.State == StateStandby {
		needed = m.confirmations
	}
	if t.pending != o.Disk.State {
		t.pending, t.pendingCount = o.Disk.State, 1
	} else {
		t.pendingCount++
	}
	if t.pendingCount < needed {
		return nil
	}

	from := t.disk.State
	to := o.Disk.State
	t.pending, t.pendingCount = "", 0
	t.disk = mergeDisk(t.disk, o.Disk, true, o.At)

	typeName := DiskActivity
	reason := "检测到磁盘状态变化"
	confidence := "中"
	if from == StateStandby && (to == StateActive || to == StateIdle) {
		typeName, reason, confidence = DiskWakeup, "可靠检测到机械盘由待机进入活动状态", "高"
	} else if to == StateStandby {
		typeName, reason, confidence = DiskStandby, "连续多次确认机械盘进入待机", "高"
	} else if to == StateUnknown {
		typeName, reason, confidence = StateUnknownEvent, "状态探测能力暂时不可用", "低"
	}
	return []Record{newRecord(t.disk, typeName, from, to, o.At, readDelta, writeDelta, o.Evidence, reason, confidence)}
}

// SetCollectorAvailable 仅在 Collector 健康状态真正变化时生成一次离线或恢复事件。
func (m *StateMachine) SetCollectorAvailable(up bool, at time.Time) []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.collectorUp == up {
		return nil
	}
	m.collectorUp = up
	t := CollectorOffline
	reason := "Collector 超过健康检查窗口未上报"
	if up {
		t, reason = CollectorRecovered, "Collector 已恢复上报"
	}
	return []Record{{Type: t, StartedAt: at, Reason: reason, Confidence: "高"}}
}

// Snapshot 在锁保护下返回当前磁盘状态副本，供低频持久化判断使用。
func (m *StateMachine) Snapshot() []Disk {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Disk, 0, len(m.disks))
	for _, t := range m.disks {
		out = append(out, t.disk)
	}
	return out
}

// mergeDisk 合并新样本和已确认状态，只在真实转换时增加唤醒次数。
func mergeDisk(old, next Disk, changed bool, at time.Time) Disk {
	next.PreviousState = old.PreviousState
	next.LastStateChange = old.LastStateChange
	if changed {
		next.PreviousState = old.State
		next.LastStateChange = at
		if old.State == StateStandby && (next.State == StateActive || next.State == StateIdle) {
			next.TodayWakeups = old.TodayWakeups + 1
		}
	} else {
		next.TodayWakeups = old.TodayWakeups
	}
	return next
}

// newRecord 组装事件，并排除应用自身 I/O 作为默认疑似来源。
func newRecord(d Disk, typ Type, from, to DiskState, at time.Time, readDelta, writeDelta uint64, evidence []Evidence, reason, confidence string) Record {
	r := Record{DiskID: d.ID, Device: d.Device, Type: typ, FromState: from, ToState: to, StartedAt: at, ReadDelta: readDelta, WriteDelta: writeDelta, Reason: reason, Confidence: confidence, Evidence: evidence}
	if len(evidence) > 0 {
		for _, e := range evidence {
			if e.SelfActivity {
				continue
			}
			r.SuspectProcess, r.SuspectApp, r.SuspectDocker = e.Process, e.FnosApp, e.Container
			if e.Reason != "" {
				r.Reason = e.Reason
			}
			if e.Confidence != "" {
				r.Confidence = e.Confidence
			}
			break
		}
	}
	return r
}

// saturatingDelta 在计数器重置或回拨时返回零，避免无符号下溢伪造巨量 I/O。
func saturatingDelta(now, before uint64) uint64 {
	if now < before {
		return 0
	}
	return now - before
}
