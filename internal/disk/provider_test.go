package disk

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestDeviceValidation 验证合法物理盘名通过且路径、分区和注入字符串被拒绝。
func TestDeviceValidation(t *testing.T) {
	for _, v := range []string{"sda", "sdaa", "hdb"} {
		if !ValidDeviceName(v) {
			t.Fatalf("should accept %s", v)
		}
	}
	for _, v := range []string{"nvme0n1", "dm-0", "sda;rm -rf", "../sda", "sda1"} {
		if ValidDeviceName(v) {
			t.Fatalf("should reject %s", v)
		}
	}
	if !scanDevicePattern.MatchString("nvme0n1") {
		t.Fatal("NVMe should be displayed as unsupported")
	}
}

// TestHDParmWhitelist 验证仅允许扫描结果中的固定 hdparm -C 调用。
func TestHDParmWhitelist(t *testing.T) {
	known := map[string]bool{"sda": true}
	if err := ValidateHDParmInvocation("/usr/sbin/hdparm", []string{"-C", "/dev/sda"}, known); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-y", "/dev/sda"}, {"-C", "/dev/sdb"}, {"-C", "/dev/sda;id"}} {
		if ValidateHDParmInvocation("hdparm", args, known) == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}

// TestMaskSerial 验证诊断和 UI 不会暴露完整序列号。
func TestMaskSerial(t *testing.T) {
	if got := MaskSerial("ABCDEF1234"); got != "******1234" {
		t.Fatal(got)
	}
}

// TestCommandTimeout 验证外部命令在上下文截止时间后被终止，不会无限阻塞 Collector。
func TestCommandTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := (execRunner{}).Run(ctx, "/bin/sleep", "1")
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}
