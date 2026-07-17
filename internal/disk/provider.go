package disk

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
)

// deviceNamePattern 只接受无分区号的传统物理块设备名称，阻止路径和 Shell 注入。
var deviceNamePattern = regexp.MustCompile(`^(sd[a-z]+|hd[a-z]+)$`)

// scanDevicePattern 额外识别 NVMe，后者只展示 unsupported 且永不进入 hdparm 白名单。
var scanDevicePattern = regexp.MustCompile(`^(sd[a-z]+|hd[a-z]+|nvme[0-9]+n[0-9]+)$`)

// CommandRunner 抽象固定外部命令执行，便于测试超时和白名单且不触碰真实硬件。
type CommandRunner interface {
	Run(ctx context.Context, command string, args ...string) ([]byte, error)
}

// execRunner 使用无 Shell 的 exec.CommandContext 执行已经校验的命令模板。
type execRunner struct{}

// limitedOutput 把外部命令合并输出限制在 64 KiB，防止异常工具消耗无界内存。
type limitedOutput struct {
	mu   sync.Mutex
	data []byte
}

// Write 在锁保护下截断超限输出，但向子进程报告已消费以允许其正常退出。
func (w *limitedOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := 64*1024 - len(w.data)
	if remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		w.data = append(w.data, p[:remaining]...)
	}
	return len(p), nil
}

// Run 执行固定命令及参数，取消上下文时终止子进程，并限制输出体积。
func (execRunner) Run(ctx context.Context, command string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	output := &limitedOutput{}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	return output.data, err
}

// Provider 定义可替换的硬件样本扫描接口，测试可使用 mock provider。
type Provider interface {
	Scan(ctx context.Context) ([]Sample, error)
}

// Sample 包含物理盘状态和来自 procfs 的累计 I/O 计数。
type Sample struct {
	// Disk 是本轮识别的物理盘及能力状态。
	Disk event.Disk `json:"disk"`
	// ReadIO 是累计读取字节数。
	ReadIO uint64 `json:"readIo"`
	// WriteIO 是累计写入字节数。
	WriteIO uint64 `json:"writeIo"`
}

// LinuxProvider 从指定 sysfs/procfs 根目录进行有界扫描。
type LinuxProvider struct {
	// SysRoot 是可替换的 sysfs 根目录，测试使用临时夹具。
	SysRoot string
	// ProcRoot 是可替换的 procfs 根目录。
	ProcRoot string
	// DevRoot 必须为 /dev 才允许执行外部硬件命令。
	DevRoot string
	// EnableHDParm 控制固定参数状态查询，默认关闭。
	EnableHDParm bool
	// Runner 是集中白名单命令执行器。
	Runner CommandRunner
	// CommandTimeout 是外部命令最大执行时间。
	CommandTimeout time.Duration
}

// NewLinuxProvider 创建 Linux Provider；硬件查询由安全配置显式控制。
func NewLinuxProvider(enableHDParm bool) *LinuxProvider {
	return &LinuxProvider{SysRoot: "/sys", ProcRoot: "/proc", DevRoot: "/dev", EnableHDParm: enableHDParm, Runner: execRunner{}, CommandTimeout: 2 * time.Second}
}

// ValidDeviceName 校验设备名为扫描支持的物理盘格式。
func ValidDeviceName(name string) bool { return deviceNamePattern.MatchString(name) }

// ValidateHDParmInvocation 验证命令、固定 -C 参数、标准设备路径和本轮扫描白名单。
func ValidateHDParmInvocation(command string, args []string, known map[string]bool) error {
	if filepath.Base(command) != "hdparm" || len(args) != 2 || args[0] != "-C" {
		return errors.New("命令或参数不在白名单")
	}
	name := filepath.Base(args[1])
	if !ValidDeviceName(name) || !known[name] || args[1] != "/dev/"+name {
		return errors.New("设备不在本次系统扫描结果中")
	}
	return nil
}

// Scan 仅读取 sysfs/procfs 元数据；硬件探测默认关闭且不会读取盘上文件。
func (p *LinuxProvider) Scan(ctx context.Context) ([]Sample, error) {
	entries, err := os.ReadDir(filepath.Join(p.SysRoot, "block"))
	if err != nil {
		return nil, err
	}
	stats, _ := readDiskstats(filepath.Join(p.ProcRoot, "diskstats"))
	known := make(map[string]bool)
	for _, e := range entries {
		if ValidDeviceName(e.Name()) {
			known[e.Name()] = true
		}
	}
	out := make([]Sample, 0, len(known))
	for _, e := range entries {
		// 单次最多扫描 128 个块设备，避免异常 sysfs 树造成无界工作量。
		if len(out) >= 128 {
			break
		}
		name := e.Name()
		if !scanDevicePattern.MatchString(name) {
			continue
		}
		base := filepath.Join(p.SysRoot, "block", name)
		rotational := strings.TrimSpace(readText(filepath.Join(base, "queue", "rotational"))) == "1"
		model := strings.TrimSpace(readText(filepath.Join(base, "device", "model")))
		serial := strings.TrimSpace(readText(filepath.Join(base, "device", "serial")))
		capacitySectors, _ := strconv.ParseUint(strings.TrimSpace(readText(filepath.Join(base, "size"))), 10, 64)
		bus := detectBus(base)
		state, method, supported := event.StateUnsupported, "media_type", false
		if rotational {
			state, method = event.StateUnknown, "capability_unavailable"
			if p.EnableHDParm {
				state, method, supported = p.powerState(ctx, name, known)
			}
		}
		id := diskID(name, model, serial)
		d := event.Disk{ID: id, Device: name, Model: model, MaskedSerial: MaskSerial(serial), CapacityBytes: capacitySectors * 512, BusType: bus, Rotational: rotational, State: state, PreviousState: event.StateUnknown, LastStateChange: time.Now(), DetectionMethod: method, CapabilitySupported: supported, Present: true}
		st := stats[name]
		out = append(out, Sample{Disk: d, ReadIO: st.readSectors * 512, WriteIO: st.writeSectors * 512})
	}
	return out, nil
}

// powerState 仅以固定参数和两秒超时查询已扫描机械盘，失败时降级为 unknown。
func (p *LinuxProvider) powerState(parent context.Context, name string, known map[string]bool) (event.DiskState, string, bool) {
	path, err := exec.LookPath("hdparm")
	if err != nil {
		return event.StateUnknown, "hdparm_unavailable", false
	}
	args := []string{"-C", filepath.Join(p.DevRoot, name)}
	// Only the real /dev root is accepted for external commands.
	if p.DevRoot != "/dev" || ValidateHDParmInvocation(path, args, known) != nil {
		return event.StateUnknown, "hdparm_rejected", false
	}
	ctx, cancel := context.WithTimeout(parent, p.CommandTimeout)
	defer cancel()
	b, err := p.Runner.Run(ctx, path, args...)
	if err != nil {
		return event.StateUnknown, "hdparm_error", false
	}
	s := strings.ToLower(string(b))
	switch {
	case strings.Contains(s, "standby"):
		return event.StateStandby, "hdparm-C", true
	case strings.Contains(s, "active/idle") || strings.Contains(s, "active"):
		return event.StateActive, "hdparm-C", true
	case strings.Contains(s, "idle"):
		return event.StateIdle, "hdparm-C", true
	default:
		return event.StateUnknown, "hdparm_unknown", false
	}
}

// diskstat 保存内核累计读写扇区数。
type diskstat struct{ readSectors, writeSectors uint64 }

// readDiskstats 解析 procfs 累计计数，不访问对应块设备。
func readDiskstats(path string) (map[string]diskstat, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]diskstat{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		r, _ := strconv.ParseUint(fields[5], 10, 64)
		w, _ := strconv.ParseUint(fields[9], 10, 64)
		out[name] = diskstat{r, w}
	}
	return out, sc.Err()
}

// readText 读取内核虚拟文件；缺失能力返回空字符串并由上层降级。
func readText(path string) string { b, _ := os.ReadFile(path); return string(b) }

// detectBus 根据 sysfs 设备链接识别总线，无法判断时返回 unknown。
func detectBus(base string) string {
	p, err := filepath.EvalSymlinks(filepath.Join(base, "device"))
	if err != nil {
		return "unknown"
	}
	s := strings.ToLower(p)
	switch {
	case strings.Contains(s, "usb"):
		return "usb"
	case strings.Contains(s, "nvme"):
		return "nvme"
	case strings.Contains(s, "ata"):
		return "sata"
	case strings.Contains(s, "sas"):
		return "sas"
	case strings.Contains(s, "virtio"):
		return "virtio"
	default:
		return "unknown"
	}
}

// diskID 对设备身份字段做不可逆短哈希，避免数据库保存完整序列号作为主键。
func diskID(name, model, serial string) string {
	sum := sha256.Sum256([]byte(name + "\x00" + model + "\x00" + serial))
	return hex.EncodeToString(sum[:8])
}

// MaskSerial 仅保留序列号末四位用于区分设备。
func MaskSerial(serial string) string {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return ""
	}
	if len(serial) <= 4 {
		return strings.Repeat("*", len(serial))
	}
	return strings.Repeat("*", len(serial)-4) + serial[len(serial)-4:]
}

// DescribeCapability 返回面向诊断的能力限制说明，不把未知描述为待机。
func DescribeCapability(d event.Disk) string {
	if !d.Rotational {
		return "非机械介质，休眠状态不适用"
	}
	if !d.CapabilitySupported {
		return fmt.Sprintf("无法无损确认状态（%s）", d.DetectionMethod)
	}
	return "使用固定参数的低风险电源状态查询；仍需针对控制器真机验证"
}
