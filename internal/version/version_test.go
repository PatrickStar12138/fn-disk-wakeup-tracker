package version

import "testing"

// TestCurrentUsesInjectedValues 验证版本接口读取构建注入值而不是独立硬编码。
func TestCurrentUsesInjectedValues(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime, oldPlatform := Version, Commit, BuildTime, Platform
	defer func() { Version, Commit, BuildTime, Platform = oldVersion, oldCommit, oldBuildTime, oldPlatform }()
	Version, Commit, BuildTime, Platform = "0.1.0", "abc123", "2026-07-17T00:00:00Z", "x86"
	got := Current()
	if got.Version != "0.1.0" || got.Commit != "abc123" || got.Platform != "x86" {
		t.Fatalf("unexpected build info: %#v", got)
	}
}
