package event

import "time"

// DiskState 表示物理磁盘状态；unknown 与 standby 始终保持不同语义。
type DiskState string

// 统一磁盘状态常量，非机械介质使用 unsupported。
const (
	StateActive      DiskState = "active"
	StateIdle        DiskState = "idle"
	StateStandby     DiskState = "standby"
	StateUnknown     DiskState = "unknown"
	StateUnsupported DiskState = "unsupported"
)

// Valid 校验状态是否属于 API 和状态机允许的固定枚举。
func (s DiskState) Valid() bool {
	switch s {
	case StateActive, StateIdle, StateStandby, StateUnknown, StateUnsupported:
		return true
	default:
		return false
	}
}

// Type 表示状态机可以持久化的事件类型。
type Type string

// 统一事件类型常量，拔插通过 state_unknown 和 disk_activity 的依据区分。
const (
	DiskWakeup         Type = "disk_wakeup"
	DiskStandby        Type = "disk_standby"
	DiskActivity       Type = "disk_activity"
	CollectorOffline   Type = "collector_offline"
	CollectorRecovered Type = "collector_recovered"
	StateUnknownEvent  Type = "state_unknown"
)

// Disk 表示物理盘身份、能力和最近一次已确认状态。
type Disk struct {
	// ID 是不含完整序列号的稳定磁盘标识。
	ID string `json:"id"`
	// Device 是经过白名单校验的内核设备名，不含 /dev 前缀。
	Device string `json:"device"`
	// Model 是 sysfs 提供的设备型号。
	Model string `json:"model"`
	// MaskedSerial 是只保留末四位的序列号。
	MaskedSerial string `json:"maskedSerial"`
	// CapacityBytes 是设备容量，单位为字节。
	CapacityBytes uint64 `json:"capacityBytes"`
	// BusType 是识别到的 SATA、SAS、USB 等总线类型。
	BusType string `json:"busType"`
	// Rotational 表示设备是否为机械旋转介质。
	Rotational bool `json:"rotational"`
	// State 是当前已确认状态，unknown 不等于 standby。
	State DiskState `json:"state"`
	// PreviousState 是上一个已确认状态。
	PreviousState DiskState `json:"previousState"`
	// LastStateChange 是最近一次已确认状态变化时间。
	LastStateChange time.Time `json:"lastStateChange"`
	// TodayWakeups 是本地日期内的已确认唤醒次数。
	TodayWakeups int `json:"todayWakeups"`
	// LastActivity 是最近一次观察到 I/O 活动的时间。
	LastActivity time.Time `json:"lastActivity"`
	// DetectionMethod 是本次状态能力使用或降级的方法。
	DetectionMethod string `json:"detectionMethod"`
	// CapabilitySupported 表示当前环境能否可靠获取电源状态。
	CapabilitySupported bool `json:"capabilitySupported"`
	// Present 表示本轮扫描是否仍能看到该物理盘。
	Present bool `json:"present"`
}

// Evidence 保存疑似来源判断所需的进程 I/O 和时间相关性证据。
type Evidence struct {
	// PID 是证据采样时的进程号。
	PID int `json:"pid"`
	// Process 是内核进程短名称。
	Process string `json:"process"`
	// Executable 是受长度约束的可执行文件路径。
	Executable string `json:"executable"`
	// CommandLine 是最多 256 字符的命令行摘要。
	CommandLine string `json:"commandLine"`
	// Cgroup 是用于推断容器的控制组摘要。
	Cgroup string `json:"cgroup"`
	// FnosApp 是从官方应用目录结构推断的应用名。
	FnosApp string `json:"fnosApp"`
	// Container 是脱敏后的容器短标识。
	Container string `json:"container"`
	// ReadDelta 是采样窗口读取增量，单位为字节。
	ReadDelta uint64 `json:"readDelta"`
	// WriteDelta 是采样窗口写入增量，单位为字节。
	WriteDelta uint64 `json:"writeDelta"`
	// Reason 是明确包含时间窗口和 I/O 增量的相关性依据。
	Reason string `json:"reason"`
	// Confidence 是高、中、低三级相关性置信度。
	Confidence string `json:"confidence"`
	// SelfActivity 表示证据来自本应用进程，默认不作为疑似来源。
	SelfActivity bool `json:"selfActivity"`
}

// Record 表示一条可持久化的状态事件及其疑似来源摘要。
type Record struct {
	// ID 是 SQLite 自增事件标识。
	ID int64 `json:"id"`
	// DiskID 是关联物理盘标识；服务事件可以为空。
	DiskID string `json:"diskId"`
	// Device 是便于 UI 展示的设备名。
	Device string `json:"device"`
	// Type 是固定事件类型。
	Type Type `json:"type"`
	// FromState 和 ToState 描述已确认转换。
	FromState DiskState `json:"fromState"`
	ToState   DiskState `json:"toState"`
	// StartedAt 是事件开始时间。
	StartedAt time.Time `json:"startedAt"`
	// EndedAt 是可选结束时间。
	EndedAt *time.Time `json:"endedAt,omitempty"`
	// DurationMS 是非负持续时间，单位为毫秒。
	DurationMS int64 `json:"durationMs"`
	// ReadDelta 和 WriteDelta 是事件窗口 I/O 增量，单位为字节。
	ReadDelta  uint64 `json:"readDelta"`
	WriteDelta uint64 `json:"writeDelta"`
	// SuspectProcess、SuspectApp、SuspectDocker 是疑似来源摘要而非确定因果。
	SuspectProcess string `json:"suspectProcess"`
	SuspectApp     string `json:"suspectFnosApp"`
	SuspectDocker  string `json:"suspectDockerContainer"`
	// Reason 是疑似来源判断依据。
	Reason string `json:"reason"`
	// Confidence 是高、中、低可信度。
	Confidence string `json:"confidence"`
	// Note 是管理员可选备注。
	Note string `json:"note"`
	// Evidence 是支撑疑似来源判断的有限证据列表。
	Evidence []Evidence `json:"evidence,omitempty"`
}
