package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/api"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/config"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/database"
	appversion "github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/version"
)

// main 启动 Server，并把启动错误写入受生命周期脚本轮换的日志。
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run 初始化 SQLite 和两个 Unix Socket；仅网关 Socket 对系统代理可读写。
func run() error {
	appDest := required("TRIM_APPDEST")
	etc := required("TRIM_PKGETC")
	data := required("TRIM_PKGVAR")
	if appDest == "" || etc == "" || data == "" {
		return errors.New("TRIM_APPDEST、TRIM_PKGETC 和 TRIM_PKGVAR 必须设置")
	}
	settingsPath := filepath.Join(etc, "settings.json")
	settings, err := config.Load(settingsPath)
	if err != nil {
		return err
	}
	if err := config.Save(settingsPath, settings); err != nil {
		return err
	}
	db, err := database.Open(filepath.Join(data, "db", "tracker.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if len(os.Args) > 1 && os.Args[1] == "--migrate-only" {
		return nil
	}
	runDir := filepath.Join(data, "run")
	if err := os.MkdirAll(runDir, 0o750); err != nil {
		return err
	}
	gatewaySocket := filepath.Join(appDest, "app.sock")
	collectorSocket := filepath.Join(runDir, "collector.sock")
	for _, p := range []string{gatewaySocket, collectorSocket} {
		if err := safeRemoveSocket(p); err != nil {
			return err
		}
	}
	gatewayListener, err := net.Listen("unix", gatewaySocket)
	if err != nil {
		return fmt.Errorf("listen gateway socket: %w", err)
	}
	defer gatewayListener.Close()
	if err := os.Chmod(gatewaySocket, 0o660); err != nil {
		return err
	}
	collectorListener, err := net.Listen("unix", collectorSocket)
	if err != nil {
		return fmt.Errorf("listen collector socket: %w", err)
	}
	defer collectorListener.Close()
	if err := os.Chmod(collectorSocket, 0o600); err != nil {
		return err
	}
	s := api.New(db, settings, settingsPath, filepath.Join(appDest, "web"), gatewaySocket, collectorSocket)
	backgroundCtx, backgroundCancel := context.WithCancel(context.Background())
	defer backgroundCancel()
	go s.Background(backgroundCtx)
	gw := &http.Server{Handler: s.GatewayHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	internal := &http.Server{Handler: s.CollectorHandler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	errCh := make(chan error, 2)
	go func() { errCh <- gw.Serve(gatewayListener) }()
	go func() { errCh <- internal.Serve(collectorListener) }()
	log.Printf("level=info module=server version=%s platform=%s commit=%s listening=unix", appversion.Version, appversion.Platform, appversion.Commit)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = internal.Shutdown(shutdown)
	_ = gw.Shutdown(shutdown)
	_ = os.Remove(collectorSocket)
	_ = os.Remove(gatewaySocket)
	return nil
}

// required 读取官方路径环境变量，由 run 统一检查空值。
func required(name string) string { return os.Getenv(name) }

// safeRemoveSocket 只删除确认是 Unix Socket 的本应用残留路径。
func safeRemoveSocket(path string) error {
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refuse to remove non-socket %s", path)
	}
	return os.Remove(path)
}
