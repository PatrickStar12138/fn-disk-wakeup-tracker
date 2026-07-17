package diagnose

import (
	"strings"
	"testing"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
)

// TestReportRedaction 验证用户名、序列号和 Authorization 错误不会进入下载报告。
func TestReportRedaction(t *testing.T) {
	r := Report{RunUser: "administrator", Disks: []event.Disk{{MaskedSerial: "ABCDEF1234"}}, RecentErrors: []string{"Authorization: secret"}}
	x := Redact(r)
	if x.RunUser == "administrator" || strings.Contains(x.Disks[0].MaskedSerial, "ABCDEF") {
		t.Fatalf("not redacted: %#v", x)
	}
	if strings.Contains(x.RecentErrors[0], "secret") {
		t.Fatal("secret leaked")
	}
}
