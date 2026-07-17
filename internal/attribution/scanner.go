package attribution

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
)

// procIO 保存进程累计读写字节数。
type procIO struct{ Read, Write uint64 }

// procSample 保存一次受限进程扫描所需的非敏感归因字段。
type procSample struct {
	PID                        int
	Name, Exe, Cmdline, Cgroup string
	IO                         procIO
}

// Scanner 以最大 PID 数和证据数限制进程相关性分析成本。
type Scanner struct {
	ProcRoot    string
	MaxPIDs     int
	MaxEvidence int
	previous    map[int]procSample
	ignored     map[string]bool
	lastScan    time.Time
}

// NewScanner 创建内存差分扫描器，并标记应用自身或管理员忽略的进程。
func NewScanner(ignored []string) *Scanner {
	m := map[string]bool{}
	for _, v := range ignored {
		m[v] = true
	}
	return &Scanner{ProcRoot: "/proc", MaxPIDs: 256, MaxEvidence: 8, previous: map[int]procSample{}, ignored: m}
}

// SetIgnored 原子替换下一轮扫描使用的忽略进程集合；调用方只在单一采集循环中调用。
func (s *Scanner) SetIgnored(ignored []string) {
	next := map[string]bool{}
	for _, value := range ignored {
		next[value] = true
	}
	s.ignored = next
}

// Scan 最多读取 256 个进程的 io/cgroup/cmdline，不遍历所有文件描述符也不写数据库。
func (s *Scanner) Scan() []event.Evidence {
	entries, err := os.ReadDir(s.ProcRoot)
	if err != nil {
		return nil
	}
	current := map[int]procSample{}
	count := 0
	for _, e := range entries {
		if count >= s.MaxPIDs {
			break
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		count++
		p := s.readPID(pid)
		if p.Name == "" {
			continue
		}
		current[pid] = p
	}
	window := time.Since(s.lastScan)
	if s.lastScan.IsZero() || window < 0 {
		window = 0
	}
	s.lastScan = time.Now()
	// ranked 仅在内存中按总 I/O 排序，保证只保留有限证据。
	type ranked struct {
		e     event.Evidence
		total uint64
	}
	list := make([]ranked, 0)
	for pid, now := range current {
		prev, ok := s.previous[pid]
		if !ok {
			continue
		}
		rd := delta(now.IO.Read, prev.IO.Read)
		wd := delta(now.IO.Write, prev.IO.Write)
		total := rd + wd
		if total == 0 {
			continue
		}
		self := s.ignored[now.Name] || strings.Contains(now.Exe, "fn-disk-wakeup-")
		confidence := Confidence(total, window, false)
		reason := fmt.Sprintf("状态变化前 %d 秒采样窗口内，进程 PID %d 读取增量 %s、写入增量 %s；该结果表示时间相关性，不代表确定因果关系。", max(1, int(window.Seconds())), pid, humanBytes(rd), humanBytes(wd))
		if self {
			reason = "应用自身 I/O 已单独标记并从默认疑似来源排行排除。 " + reason
			confidence = "低"
		}
		list = append(list, ranked{event.Evidence{PID: pid, Process: now.Name, Executable: now.Exe, CommandLine: safeCmdline(now.Cmdline), Cgroup: now.Cgroup, FnosApp: inferApp(now.Exe), Container: inferContainer(now.Cgroup), ReadDelta: rd, WriteDelta: wd, Reason: reason, Confidence: confidence, SelfActivity: self}, total})
	}
	s.previous = current
	sort.Slice(list, func(i, j int) bool { return list[i].total > list[j].total })
	if len(list) > s.MaxEvidence {
		list = list[:s.MaxEvidence]
	}
	out := make([]event.Evidence, 0, len(list))
	for _, v := range list {
		out = append(out, v.e)
	}
	return out
}

// readPID 从 procfs 读取单个 PID 的有限字段，进程消失时返回空样本。
func (s *Scanner) readPID(pid int) procSample {
	base := filepath.Join(s.ProcRoot, strconv.Itoa(pid))
	name := strings.TrimSpace(read(filepath.Join(base, "comm")))
	exe, _ := os.Readlink(filepath.Join(base, "exe"))
	cmd := strings.ReplaceAll(read(filepath.Join(base, "cmdline")), "\x00", " ")
	cg := read(filepath.Join(base, "cgroup"))
	io := parseIO(read(filepath.Join(base, "io")))
	return procSample{PID: pid, Name: name, Exe: exe, Cmdline: strings.TrimSpace(cmd), Cgroup: strings.TrimSpace(cg), IO: io}
}

// parseIO 解析 procfs 的累计 read_bytes 和 write_bytes。
func parseIO(v string) procIO {
	var out procIO
	sc := bufio.NewScanner(strings.NewReader(v))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 2 {
			continue
		}
		n, _ := strconv.ParseUint(f[1], 10, 64)
		switch strings.TrimSuffix(f[0], ":") {
		case "read_bytes":
			out.Read = n
		case "write_bytes":
			out.Write = n
		}
	}
	return out
}

// Confidence 根据 I/O 量、时间窗口和可选 FD 证据给出相关性等级而非确定因果。
func Confidence(bytes uint64, window time.Duration, hasFD bool) string {
	if hasFD && bytes >= 1024*1024 {
		return "高"
	}
	if bytes >= 16*1024*1024 && window <= 10*time.Second {
		return "高"
	}
	if bytes >= 1024*1024 && window <= 30*time.Second {
		return "中"
	}
	return "低"
}

// inferApp 从官方安装目标结构中推断 fnOS 应用名，不硬编码具体安装卷。
func inferApp(exe string) string {
	parts := strings.Split(filepath.Clean(exe), string(filepath.Separator))
	for i, p := range parts {
		if p == "@appcenter" && i+1 < len(parts) {
			return parts[i+1]
		}
		if p == "target" && i > 0 {
			return parts[i-1]
		}
	}
	return ""
}

// inferContainer 从 cgroup 中提取脱敏的十二位容器标识。
func inferContainer(cgroup string) string {
	for _, line := range strings.Split(cgroup, "\n") {
		for _, part := range strings.Split(line, "/") {
			v := strings.TrimSuffix(strings.TrimPrefix(part, "docker-"), ".scope")
			if len(v) >= 12 && isHex(v) {
				return v[:12]
			}
		}
	}
	return ""
}

// safeCmdline 限制命令行证据长度，避免诊断和数据库无界增长。
func safeCmdline(v string) string {
	if len(v) > 256 {
		return v[:256] + "…"
	}
	return v
}

// isHex 校验容器标识只含十六进制字符。
func isHex(v string) bool {
	for _, r := range v {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

// humanBytes 把字节数转换为证据文本中的易读单位。
func humanBytes(v uint64) string {
	const mb = 1024 * 1024
	if v >= mb {
		return fmt.Sprintf("%.1f MB", float64(v)/mb)
	}
	if v >= 1024 {
		return fmt.Sprintf("%.1f KB", float64(v)/1024)
	}
	return fmt.Sprintf("%d B", v)
}

// delta 在进程计数器重置时返回零，防止无符号下溢。
func delta(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}

// read 读取 procfs 虚拟文件，权限或竞态失败时返回空值并放弃该证据。
func read(path string) string { b, _ := os.ReadFile(path); return string(b) }

// max 返回两个整数中的较大值，用于保证证据窗口至少显示一秒。
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
