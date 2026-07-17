package event

import (
	"testing"
	"time"
)

// disk 构造不接触真实硬件的测试磁盘样本。
func disk(id string, state DiskState, present bool) Disk {
	return Disk{ID: id, Device: "sd" + id, State: state, Rotational: true, Present: present}
}

// TestFirstStartupDoesNotWake 验证首次活动样本不会被误记为唤醒。
func TestFirstStartupDoesNotWake(t *testing.T) {
	m := NewStateMachine(2)
	if got := m.Observe(Observation{Disk: disk("a", StateActive, true), At: time.Now()}); len(got) != 0 {
		t.Fatalf("got %d events", len(got))
	}
}

// TestStandbyToActiveImmediate 验证可靠的待机到活动转换立即产生唤醒事件。
func TestStandbyToActiveImmediate(t *testing.T) {
	m := NewStateMachine(2)
	now := time.Now()
	m.Seed([]Disk{disk("a", StateStandby, true)})
	got := m.Observe(Observation{Disk: disk("a", StateActive, true), At: now})
	if len(got) != 1 || got[0].Type != DiskWakeup {
		t.Fatalf("unexpected events: %#v", got)
	}
}

// TestActiveToStandbyRequiresConfirmation 验证待机必须达到连续确认次数。
func TestActiveToStandbyRequiresConfirmation(t *testing.T) {
	m := NewStateMachine(2)
	m.Seed([]Disk{disk("a", StateActive, true)})
	if got := m.Observe(Observation{Disk: disk("a", StateStandby, true)}); len(got) != 0 {
		t.Fatal("premature event")
	}
	got := m.Observe(Observation{Disk: disk("a", StateStandby, true)})
	if len(got) != 1 || got[0].Type != DiskStandby {
		t.Fatalf("unexpected: %#v", got)
	}
}

// TestRepeatedAndFlappingState 验证重复状态和短暂抖动不会写入事件。
func TestRepeatedAndFlappingState(t *testing.T) {
	m := NewStateMachine(3)
	m.Seed([]Disk{disk("a", StateActive, true)})
	for _, s := range []DiskState{StateActive, StateStandby, StateActive, StateStandby, StateActive} {
		if got := m.Observe(Observation{Disk: disk("a", s, true)}); len(got) != 0 {
			t.Fatalf("unexpected event for %s", s)
		}
	}
}

// TestRestartSeedDoesNotWake 验证恢复数据库状态后不会产生重启伪唤醒。
func TestRestartSeedDoesNotWake(t *testing.T) {
	m := NewStateMachine(2)
	m.Seed([]Disk{disk("a", StateActive, true)})
	if got := m.Observe(Observation{Disk: disk("a", StateActive, true)}); len(got) != 0 {
		t.Fatal("restart created event")
	}
}

// TestDisconnectAndReconnectAreNotWakeup 验证拔出和重新接入与普通唤醒明确区分。
func TestDisconnectAndReconnectAreNotWakeup(t *testing.T) {
	m := NewStateMachine(1)
	m.Seed([]Disk{disk("a", StateStandby, true)})
	got := m.Observe(Observation{Disk: disk("a", StateUnknown, false)})
	if len(got) != 1 || got[0].Type != StateUnknownEvent {
		t.Fatalf("disconnect: %#v", got)
	}
	got = m.Observe(Observation{Disk: disk("a", StateActive, true)})
	if len(got) != 1 || got[0].Type != DiskActivity {
		t.Fatalf("reconnect: %#v", got)
	}
}

// TestCounterRollbackDoesNotUnderflow 验证时间或 I/O 计数回拨不会产生负时长或溢出。
func TestCounterRollbackDoesNotUnderflow(t *testing.T) {
	m := NewStateMachine(1)
	m.Observe(Observation{Disk: disk("a", StateStandby, true), ReadIO: 100, WriteIO: 100})
	got := m.Observe(Observation{Disk: disk("a", StateActive, true), ReadIO: 10, WriteIO: 20, At: time.Now().Add(-time.Hour)})
	if got[0].ReadDelta != 0 || got[0].WriteDelta != 0 || got[0].DurationMS < 0 {
		t.Fatalf("negative/underflow result: %#v", got[0])
	}
}

// TestMultipleDisksAndCollectorTransitions 验证多盘独立处理及 Collector 状态防重复。
func TestMultipleDisksAndCollectorTransitions(t *testing.T) {
	m := NewStateMachine(1)
	m.Seed([]Disk{disk("a", StateStandby, true), disk("b", StateStandby, true)})
	if len(m.Observe(Observation{Disk: disk("a", StateActive, true)})) != 1 || len(m.Observe(Observation{Disk: disk("b", StateActive, true)})) != 1 {
		t.Fatal("expected independent wakeups")
	}
	if len(m.SetCollectorAvailable(false, time.Now())) != 1 || len(m.SetCollectorAvailable(false, time.Now())) != 0 || len(m.SetCollectorAvailable(true, time.Now())) != 1 {
		t.Fatal("collector transition debounce failed")
	}
}
