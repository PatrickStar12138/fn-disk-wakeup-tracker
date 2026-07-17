package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/api"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/attribution"
	collectorclient "github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/collector"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/config"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/disk"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
)

// errorLimiter 把重复采集错误日志限制为每五分钟同类最多一条。
type errorLimiter struct {
	// last 保存每个错误类别最近一次实际写日志的时间。
	last map[string]time.Time
}

// ShouldLog 判断指定错误类别是否已超过最小日志间隔。
func (l *errorLimiter) ShouldLog(category string, now time.Time) bool {
	if previous := l.last[category]; !previous.IsZero() && now.Sub(previous) < 5*time.Minute {
		return false
	}
	l.last[category] = now
	return true
}

// main 运行单一可取消采集循环；设置每轮安全重读且不会启动重复 goroutine。
func main() {
	data, settingsDir := os.Getenv("TRIM_PKGVAR"), os.Getenv("TRIM_PKGETC")
	if data == "" || settingsDir == "" {
		log.Fatal("TRIM_PKGVAR and TRIM_PKGETC are required")
	}
	settingsPath := filepath.Join(settingsDir, "settings.json")
	settings, err := config.Load(settingsPath)
	if err != nil {
		log.Fatal(err)
	}
	provider := disk.NewLinuxProvider(settings.EnableHDParmProbe)
	scanner := attribution.NewScanner(settings.IgnoredProcesses)
	client := collectorclient.NewClient(filepath.Join(data, "run", "collector.sock"))
	errors := &errorLimiter{last: map[string]time.Time{}}
	knownDisks := map[string]event.Disk{}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if latest, err := config.Load(settingsPath); err == nil {
				settings = latest
				provider.EnableHDParm = settings.EnableHDParmProbe
				scanner.SetIgnored(settings.IgnoredProcesses)
			}
			samples, err := provider.Scan(ctx)
			if err != nil {
				if errors.ShouldLog("scan", time.Now()) {
					log.Printf("level=warn module=collector scan unavailable: %v", err)
				}
			} else {
				evidence := scanner.Scan()
				payload := api.CollectorPayload{SentAt: time.Now(), Samples: make([]event.Observation, 0, len(samples))}
				currentDisks := make(map[string]event.Disk, len(samples))
				for _, v := range samples {
					currentDisks[v.Disk.ID] = v.Disk
					ev := evidence
					if v.Disk.State == event.StateStandby || v.Disk.State == event.StateUnsupported {
						ev = nil
					}
					payload.Samples = append(payload.Samples, event.Observation{Disk: v.Disk, At: time.Now(), ReadIO: v.ReadIO, WriteIO: v.WriteIO, Evidence: ev})
				}
				// 对上一轮存在但本轮消失的磁盘发送明确离线样本，区分 USB 拔出和普通唤醒。
				for id, previous := range knownDisks {
					if _, present := currentDisks[id]; present {
						continue
					}
					previous.Present, previous.State = false, event.StateUnknown
					payload.Samples = append(payload.Samples, event.Observation{Disk: previous, At: time.Now()})
				}
				knownDisks = currentDisks
				sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				if err := client.Send(sendCtx, payload); err != nil {
					if errors.ShouldLog("server", time.Now()) {
						log.Printf("level=warn module=collector server unavailable: %v", err)
					}
				}
				cancel()
			}
			timer.Reset(time.Duration(settings.SampleIntervalSeconds) * time.Second)
		}
	}
}
