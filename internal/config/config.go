package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// 配置边界常量限制采样和保留任务的资源消耗。
const (
	MinSampleInterval = 5
	MaxSampleInterval = 300
	MinRetentionDays  = 1
	MaxRetentionDays  = 365
)

// Settings 表示管理员可修改且必须由后端再次校验的运行设置。
type Settings struct {
	// SampleIntervalSeconds 是采样间隔，单位为秒。
	SampleIntervalSeconds int `json:"sampleIntervalSeconds"`
	// StateConfirmations 是进入待机前所需的连续确认次数。
	StateConfirmations int `json:"stateConfirmations"`
	// RetentionDays 是事件和操作记录保留天数。
	RetentionDays int `json:"retentionDays"`
	// MaxDatabaseMB 是 SQLite 文件允许的最大体积，单位为 MB。
	MaxDatabaseMB int `json:"maxDatabaseMB"`
	// LogLevel 是运行日志最低级别。
	LogLevel string `json:"logLevel"`
	// LogRetentionFiles 是日志轮换保留文件数。
	LogRetentionFiles int `json:"logRetentionFiles"`
	// RecordLowConfidence 控制是否保留低可信度来源。
	RecordLowConfidence bool `json:"recordLowConfidence"`
	// ShowMaskedSerial 控制 UI 是否显示脱敏序列号。
	ShowMaskedSerial bool `json:"showMaskedSerial"`
	// DefaultTimeRange 是 UI 默认查询范围。
	DefaultTimeRange string `json:"defaultTimeRange"`
	// IgnoredProcesses 是归因时忽略或单独标记的进程名。
	IgnoredProcesses []string `json:"ignoredProcesses"`
	// EnableHDParmProbe 控制潜在硬件状态查询，安全默认值为关闭。
	EnableHDParmProbe bool `json:"enableHdparmProbe"`
}

// Defaults 返回偏保守的默认设置，不启用尚未真机验证的硬件查询。
func Defaults() Settings {
	return Settings{
		SampleIntervalSeconds: 15,
		StateConfirmations:    3,
		RetentionDays:         30,
		MaxDatabaseMB:         200,
		LogLevel:              "info",
		LogRetentionFiles:     5,
		RecordLowConfidence:   true,
		ShowMaskedSerial:      true,
		DefaultTimeRange:      "24h",
		IgnoredProcesses:      []string{"fn-disk-wakeup-server", "fn-disk-wakeup-collector"},
		EnableHDParmProbe:     false,
	}
}

// Validate 校验所有数值范围和枚举，阻止高频采样或无界保留配置。
func (s Settings) Validate() error {
	if s.SampleIntervalSeconds < MinSampleInterval || s.SampleIntervalSeconds > MaxSampleInterval {
		return fmt.Errorf("采样间隔必须在 %d 到 %d 秒之间", MinSampleInterval, MaxSampleInterval)
	}
	if s.StateConfirmations < 1 || s.StateConfirmations > 10 {
		return errors.New("状态确认次数必须在 1 到 10 之间")
	}
	if s.RetentionDays < MinRetentionDays || s.RetentionDays > MaxRetentionDays {
		return fmt.Errorf("保留天数必须在 %d 到 %d 天之间", MinRetentionDays, MaxRetentionDays)
	}
	if s.MaxDatabaseMB < 20 || s.MaxDatabaseMB > 2048 {
		return errors.New("数据库上限必须在 20 到 2048 MB 之间")
	}
	if s.LogRetentionFiles < 1 || s.LogRetentionFiles > 20 {
		return errors.New("日志保留数量必须在 1 到 20 之间")
	}
	if s.LogLevel != "error" && s.LogLevel != "warn" && s.LogLevel != "info" && s.LogLevel != "debug" {
		return errors.New("日志级别无效")
	}
	if s.DefaultTimeRange != "24h" && s.DefaultTimeRange != "7d" && s.DefaultTimeRange != "30d" {
		return errors.New("默认时间范围无效")
	}
	if len(s.IgnoredProcesses) > 100 {
		return errors.New("忽略进程最多 100 项")
	}
	for _, name := range s.IgnoredProcesses {
		if len(name) == 0 || len(name) > 128 {
			return errors.New("忽略进程名称长度无效")
		}
	}
	return nil
}

// Load 从应用配置目录读取设置；文件不存在时返回安全默认值且不写盘。
func Load(path string) (Settings, error) {
	s := Defaults()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("解析设置: %w", err)
	}
	return s, s.Validate()
}

// Save 校验后通过同目录临时文件原子替换设置，避免部分写入损坏配置。
func Save(path string, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
