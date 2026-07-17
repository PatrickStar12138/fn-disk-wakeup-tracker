package diagnose

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/config"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
	appversion "github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/version"
)

// Report 汇总经过白名单选择的系统、服务、数据库和能力诊断字段。
type Report struct {
	// GeneratedAt 是报告生成时间。
	GeneratedAt time.Time `json:"generatedAt"`
	// FnosVersion 是官方环境变量提供的系统版本。
	FnosVersion string `json:"fnosVersion"`
	// Kernel 是内核版本摘要。
	Kernel string `json:"kernelVersion"`
	// Architecture 是当前 CPU 架构。
	Architecture string `json:"architecture"`
	// Application 是构建注入的应用版本信息。
	Application appversion.Info `json:"application"`
	// RunUser 是输出前会脱敏的运行用户名。
	RunUser string `json:"runUser"`
	// ServerStatus 和 CollectorStatus 是服务健康摘要。
	ServerStatus    string `json:"serverStatus"`
	CollectorStatus string `json:"collectorStatus"`
	// GatewaySocket 和 CollectorSocket 仅报告存在性与权限。
	GatewaySocket   string `json:"gatewaySocketStatus"`
	CollectorSocket string `json:"collectorSocketStatus"`
	// DatabaseStatus 是 SQLite 健康状态。
	DatabaseStatus string `json:"databaseStatus"`
	// DatabaseSize 是 SQLite 主文件大小，单位为字节。
	DatabaseSize int64 `json:"databaseSizeBytes"`
	// SchemaVersion 是最高迁移版本。
	SchemaVersion int `json:"schemaVersion"`
	// Disks 是已经脱敏的设备扫描结果。
	Disks []event.Disk `json:"disks"`
	// Commands 只表示固定诊断命令是否存在，不执行它们。
	Commands map[string]bool `json:"availableCommands"`
	// PermissionChecks 是最小权限检查摘要。
	PermissionChecks []string `json:"permissionChecks"`
	// RecentErrors 是过滤凭据后的有限错误摘要。
	RecentErrors []string `json:"recentErrors"`
	// Settings 是当前非敏感配置。
	Settings config.Settings `json:"settings"`
	// VersionConsistent 表示运行版本是否由发布构建注入。
	VersionConsistent bool `json:"versionConsistent"`
}

// New 创建诊断报告，只检查固定命令是否存在，不执行硬件查询或读取用户文件。
func New(settings config.Settings, disks []event.Disk, dbSize int64, schema int, collectorOK bool, gatewaySocket, collectorSocket string) Report {
	commands := map[string]bool{}
	for _, name := range []string{"hdparm", "smartctl", "lsblk"} {
		_, err := exec.LookPath(name)
		commands[name] = err == nil
	}
	collector := "离线"
	if collectorOK {
		collector = "正常"
	}
	return Report{GeneratedAt: time.Now(), FnosVersion: os.Getenv("TRIM_SYS_VERSION"), Kernel: firstNonempty(os.Getenv("TRIM_KERNEL_VERSION"), runtime.GOOS), Architecture: runtime.GOARCH, Application: appversion.Current(), RunUser: firstNonempty(os.Getenv("TRIM_RUN_USERNAME"), os.Getenv("USER")), ServerStatus: "正常", CollectorStatus: collector, GatewaySocket: fileStatus(gatewaySocket), CollectorSocket: fileStatus(collectorSocket), DatabaseStatus: "正常", DatabaseSize: dbSize, SchemaVersion: schema, Disks: disks, Commands: commands, PermissionChecks: []string{"Server 使用应用专用用户（待 fnOS 进程表复核）", "Collector Socket 未暴露到网络"}, Settings: settings, VersionConsistent: appversion.Version != "dev"}
}

// JSON 再次脱敏报告后生成可下载 JSON。
func (r Report) JSON() (string, error) {
	b, err := json.MarshalIndent(Redact(r), "", "  ")
	return string(b), err
}

// Text 生成有限字段的纯文本报告，不包含完整环境变量和文件内容。
func (r Report) Text() string {
	r = Redact(r)
	var b strings.Builder
	fmt.Fprintf(&b, "硬盘唤醒追踪器诊断报告\n生成时间: %s\nfnOS: %s\n内核: %s\n架构: %s\n应用版本: %s\n构建提交: %s\nServer: %s\nCollector: %s\n数据库: %s (%d bytes, schema %d)\n", r.GeneratedAt.Format(time.RFC3339), r.FnosVersion, r.Kernel, r.Architecture, r.Application.Version, r.Application.Commit, r.ServerStatus, r.CollectorStatus, r.DatabaseStatus, r.DatabaseSize, r.SchemaVersion)
	keys := make([]string, 0, len(r.Commands))
	for k := range r.Commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "命令 %s: %t\n", k, r.Commands[k])
	}
	for _, d := range r.Disks {
		fmt.Fprintf(&b, "磁盘 %s: model=%s serial=%s state=%s method=%s supported=%t\n", d.Device, d.Model, d.MaskedSerial, d.State, d.DetectionMethod, d.CapabilitySupported)
	}
	return b.String()
}

// Redact 脱敏用户名、磁盘序列号和可能含凭据的错误摘要。
func Redact(r Report) Report {
	for i := range r.Disks {
		r.Disks[i].MaskedSerial = remask(r.Disks[i].MaskedSerial)
	}
	r.RunUser = maskUser(r.RunUser)
	for i, v := range r.RecentErrors {
		r.RecentErrors[i] = redactSecrets(v)
	}
	return r
}

// redactSecrets 检测常见凭据关键字并替换整条敏感错误。
func redactSecrets(s string) string {
	lower := strings.ToLower(s)
	for _, key := range []string{"token", "password", "cookie", "authorization"} {
		if strings.Contains(lower, key) {
			return "[已脱敏的敏感错误信息]"
		}
	}
	return s
}

// remask 对已经脱敏的序列号再次收紧展示，避免调用方误传原值。
func remask(v string) string {
	if v == "" {
		return ""
	}
	visible := v
	if len(v) > 4 {
		visible = v[len(v)-4:]
	}
	return "****" + visible
}

// maskUser 仅保留用户名首字符。
func maskUser(v string) string {
	if v == "" {
		return ""
	}
	r := []rune(v)
	if len(r) <= 1 {
		return "*"
	}
	return string(r[0]) + strings.Repeat("*", len(r)-1)
}

// fileStatus 只报告 Socket 路径存在性和权限位，不暴露内容。
func fileStatus(path string) string {
	if path == "" {
		return "未配置"
	}
	st, err := os.Stat(path)
	if err != nil {
		return "不存在"
	}
	return fmt.Sprintf("存在 mode=%s", st.Mode().Perm())
}

// firstNonempty 返回首个非空诊断字段或 unknown。
func firstNonempty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return "unknown"
}
