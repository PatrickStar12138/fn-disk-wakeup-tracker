package attribution

import (
	"testing"
	"time"
)

// TestConfidence 验证高、中、低可信度阈值不会把弱相关性夸大为高可信度。
func TestConfidence(t *testing.T) {
	if Confidence(20*1024*1024, 5*time.Second, false) != "高" {
		t.Fatal("high")
	}
	if Confidence(2*1024*1024, 20*time.Second, false) != "中" {
		t.Fatal("medium")
	}
	if Confidence(100, 5*time.Second, false) != "低" {
		t.Fatal("low")
	}
}

// TestInference 验证 fnOS 应用与容器信息从受控路径和 cgroup 正确脱敏推断。
func TestInference(t *testing.T) {
	if got := inferApp("/some/volume/@appcenter/example/bin/run"); got != "example" {
		t.Fatal(got)
	}
	if got := inferContainer("0::/docker/0123456789abcdef"); got != "0123456789ab" {
		t.Fatal(got)
	}
}
