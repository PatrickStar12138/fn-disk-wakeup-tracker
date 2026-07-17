package version

// 构建信息由 build.sh 使用 ldflags 注入，源码不维护发布版本号。
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	Platform  = "unknown"
)

// Info 是版本 API 和诊断报告共享的构建信息。
type Info struct {
	// Version 是 VERSION 注入的发布版本。
	Version string `json:"version"`
	// Commit 是构建时 Git 提交短哈希。
	Commit string `json:"commit"`
	// BuildTime 是 UTC RFC3339 构建时间。
	BuildTime string `json:"buildTime"`
	// Platform 是 fnOS manifest 平台值 x86 或 arm。
	Platform string `json:"platform"`
}

// Current 返回当前进程内由构建参数注入的版本信息。
func Current() Info {
	return Info{Version: Version, Commit: Commit, BuildTime: BuildTime, Platform: Platform}
}
