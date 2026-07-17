package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// User 表示由可信 fnOS 统一网关转发的管理员身份上下文。
type User struct {
	// UID 是统一网关转发的当前用户 ID。
	UID string `json:"uid"`
	// Username 是统一网关转发的当前用户名。
	Username string `json:"username"`
	// IsAdmin 表示用户是否通过管理员校验。
	IsAdmin bool `json:"isAdmin"`
}

// key 是私有上下文键，避免与其他中间件发生键冲突。
type key struct{}

// UserFromContext 读取已由 AdminOnly 校验的用户身份。
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(key{}).(User)
	return u, ok
}

// AdminOnly 要求三个网关 Header 完整且管理员标志严格为 true。
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := strings.TrimSpace(r.Header.Get("X-Trim-Userid"))
		username := strings.TrimSpace(r.Header.Get("X-Trim-Username"))
		admin := strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Trim-Isadmin")), "true")
		if uid == "" || username == "" {
			writeError(w, http.StatusUnauthorized, "缺少可信的飞牛网关用户上下文")
			return
		}
		if !admin {
			writeError(w, http.StatusForbidden, "仅管理员可以访问硬盘管理工具")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, User{UID: uid, Username: username, IsAdmin: true})))
	})
}

// writeError 输出不含内部实现细节的统一 JSON 鉴权错误。
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg, "status": status})
}
