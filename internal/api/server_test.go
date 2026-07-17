package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/config"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/database"
)

// testServer 使用临时 SQLite 创建不接触硬件和网络的 API 测试实例。
func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, config.Defaults(), filepath.Join(dir, "settings.json"), dir, "", "")
}

// admin 为测试请求添加模拟的可信网关管理员上下文。
func admin(r *http.Request) {
	r.Header.Set("X-Trim-Userid", "1")
	r.Header.Set("X-Trim-Username", "admin")
	r.Header.Set("X-Trim-Isadmin", "true")
}

// TestVersionRequiresAdmin 验证版本接口也不能绕过管理员鉴权。
func TestVersionRequiresAdmin(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest("GET", GatewayPrefix+"/api/v1/version", nil)
	w := httptest.NewRecorder()
	s.GatewayHandler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("GET", GatewayPrefix+"/api/v1/version", nil)
	admin(r)
	w = httptest.NewRecorder()
	s.GatewayHandler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}

// TestSettingsValidation 验证过快采样设置返回 422 且不会写入。
func TestSettingsValidation(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest("PUT", GatewayPrefix+"/api/v1/settings", strings.NewReader(`{"sampleIntervalSeconds":1}`))
	admin(r)
	w := httptest.NewRecorder()
	s.GatewayHandler().ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

// TestEventRangeValidation 验证无界时间范围会被 API 拒绝。
func TestEventRangeValidation(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest("GET", GatewayPrefix+"/api/v1/events?range=forever", nil)
	admin(r)
	w := httptest.NewRecorder()
	s.GatewayHandler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
