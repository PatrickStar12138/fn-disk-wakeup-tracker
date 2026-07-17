package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAdminOnly 验证缺失身份返回 401、普通用户返回 403、管理员允许访问。
func TestAdminOnly(t *testing.T) {
	h := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		name, uid, user, admin string
		want                   int
	}{{"missing", "", "", "", 401}, {"user", "1", "bob", "false", 403}, {"admin", "1", "root", "true", 204}} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("X-Trim-Userid", tc.uid)
			r.Header.Set("X-Trim-Username", tc.user)
			r.Header.Set("X-Trim-Isadmin", tc.admin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d", w.Code)
			}
		})
	}
}
