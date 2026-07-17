package config

import "testing"

// TestDefaultsAreValid 验证出厂设置满足全部安全边界。
func TestDefaultsAreValid(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestSettingsBounds 验证过快采样和过大数据库上限会被后端拒绝。
func TestSettingsBounds(t *testing.T) {
	s := Defaults()
	s.SampleIntervalSeconds = 1
	if s.Validate() == nil {
		t.Fatal("expected interval validation error")
	}
	s = Defaults()
	s.MaxDatabaseMB = 4096
	if s.Validate() == nil {
		t.Fatal("expected database limit validation error")
	}
}
