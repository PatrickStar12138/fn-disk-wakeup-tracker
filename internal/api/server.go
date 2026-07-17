package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/auth"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/config"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/database"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/diagnose"
	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
	appversion "github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/version"
)

const GatewayPrefix = "/app/fn-disk-wakeup-tracker"

// Server 组合管理员网关 API、静态 UI、内存状态机和低频数据库任务。
type Server struct {
	store           *database.Store
	machine         *event.StateMachine
	settingsMu      sync.RWMutex
	settings        config.Settings
	settingsPath    string
	webDir          string
	gatewaySocket   string
	collectorSocket string
	collectorMu     sync.RWMutex
	lastCollector   time.Time
	lastRefresh     time.Time
	mutations       *limiter
	startedAt       time.Time
}

// CollectorPayload 是特权 Collector 通过内部 Unix Socket 上报的有限批次。
type CollectorPayload struct {
	// Samples 是单批最多 256 个磁盘样本（含上一轮消失设备）。
	Samples []event.Observation `json:"samples"`
	// SentAt 是 Collector 发送批次的时间。
	SentAt time.Time `json:"sentAt"`
}

// New 创建 API Server，并从数据库恢复状态机基线以抑制重启伪唤醒。
func New(store *database.Store, settings config.Settings, settingsPath, webDir, gatewaySocket, collectorSocket string) *Server {
	s := &Server{store: store, machine: event.NewStateMachine(settings.StateConfirmations), settings: settings, settingsPath: settingsPath, webDir: webDir, gatewaySocket: gatewaySocket, collectorSocket: collectorSocket, mutations: newLimiter(2 * time.Second), startedAt: time.Now()}
	if disks, err := store.Disks(context.Background()); err == nil {
		s.machine.Seed(disks)
	}
	return s
}

// GatewayHandler 注册统一网关子路径，所有页面和 API 都要求管理员身份。
func (s *Server) GatewayHandler() http.Handler {
	mux := http.NewServeMux()
	apiPrefix := GatewayPrefix + "/api/v1"
	mux.Handle("GET "+apiPrefix+"/version", auth.AdminOnly(http.HandlerFunc(s.version)))
	mux.Handle("GET "+apiPrefix+"/overview", auth.AdminOnly(http.HandlerFunc(s.overview)))
	mux.Handle("GET "+apiPrefix+"/disks", auth.AdminOnly(http.HandlerFunc(s.disks)))
	mux.Handle("GET "+apiPrefix+"/events", auth.AdminOnly(http.HandlerFunc(s.events)))
	mux.Handle("GET "+apiPrefix+"/events/export.csv", auth.AdminOnly(http.HandlerFunc(s.exportCSV)))
	mux.Handle("GET "+apiPrefix+"/settings", auth.AdminOnly(http.HandlerFunc(s.getSettings)))
	mux.Handle("PUT "+apiPrefix+"/settings", auth.AdminOnly(s.mutation(http.HandlerFunc(s.putSettings))))
	mux.Handle("POST "+apiPrefix+"/refresh", auth.AdminOnly(s.mutation(http.HandlerFunc(s.refresh))))
	mux.Handle("GET "+apiPrefix+"/diagnostics", auth.AdminOnly(http.HandlerFunc(s.diagnostics)))
	mux.Handle("GET "+apiPrefix+"/diagnostics.txt", auth.AdminOnly(http.HandlerFunc(s.diagnosticsText)))
	mux.Handle("GET "+apiPrefix+"/diagnostics.json", auth.AdminOnly(http.HandlerFunc(s.diagnosticsJSON)))
	mux.Handle(GatewayPrefix+"/", auth.AdminOnly(s.spa()))
	mux.Handle(GatewayPrefix, auth.AdminOnly(http.RedirectHandler(GatewayPrefix+"/", http.StatusTemporaryRedirect)))
	return securityHeaders(mux)
}

// CollectorHandler 只供权限为 0600 的内部 Unix Socket 使用，不监听网络端口。
func (s *Server) CollectorHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /collect", s.collect)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"ok": true}) })
	return mux
}

// collect 接收有界样本；状态无变化时仅更新内存，不写 SQLite。
func (s *Server) collect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	defer r.Body.Close()
	var p CollectorPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeAPIError(w, 400, "采集数据无效")
		return
	}
	if len(p.Samples) > 256 {
		writeAPIError(w, 413, "单批磁盘样本过多")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	count := 0
	for _, o := range p.Samples {
		if o.Disk.ID == "" || !o.Disk.State.Valid() {
			continue
		}
		s.settingsMu.RLock()
		recordLow := s.settings.RecordLowConfidence
		s.settingsMu.RUnlock()
		if !recordLow {
			o.Evidence = filterLowConfidence(o.Evidence)
		}
		before, existed := findDisk(s.machine.Snapshot(), o.Disk.ID)
		records := s.machine.Observe(o)
		for _, rec := range records {
			if err := s.store.InsertEvent(ctx, &rec); err == nil {
				count++
			}
		}
		current, _ := findDisk(s.machine.Snapshot(), o.Disk.ID)
		if existed && len(records) == 0 && samePersistentDisk(before, current) {
			continue
		}
		if err := s.store.UpsertDisk(ctx, current); err != nil {
			writeAPIError(w, 500, "无法保存状态变化")
			return
		}
		_ = s.store.UpsertPowerCapability(ctx, current, capabilityDetail(current))
	}
	s.collectorMu.Lock()
	wasOffline := !s.lastCollector.IsZero() && time.Since(s.lastCollector) > 2*time.Minute
	s.lastCollector = time.Now()
	s.lastRefresh = time.Now()
	s.collectorMu.Unlock()
	if wasOffline {
		for _, rec := range s.machine.SetCollectorAvailable(true, time.Now()) {
			_ = s.store.InsertEvent(ctx, &rec)
		}
	}
	writeJSON(w, 200, map[string]any{"accepted": len(p.Samples), "events": count})
}

// Background 监测 Collector 健康并每日至多执行一次聚合清理，ctx 取消即退出。
func (s *Server) Background(ctx context.Context) {
	health := time.NewTicker(30 * time.Second)
	cleanup := time.NewTimer(6 * time.Hour)
	defer health.Stop()
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-health.C:
			s.collectorMu.RLock()
			last := s.lastCollector
			s.collectorMu.RUnlock()
			if (last.IsZero() && now.Sub(s.startedAt) > 2*time.Minute) || (!last.IsZero() && now.Sub(last) > 2*time.Minute) {
				for _, rec := range s.machine.SetCollectorAvailable(false, now) {
					_ = s.store.InsertEvent(ctx, &rec)
				}
			}
		case <-cleanup.C:
			s.settingsMu.RLock()
			settings := s.settings
			s.settingsMu.RUnlock()
			_ = s.store.AggregateDaily(ctx, time.Now().Add(-24*time.Hour))
			_, _ = s.store.Cleanup(ctx, settings.RetentionDays, settings.MaxDatabaseMB)
			cleanup.Reset(24 * time.Hour)
		}
	}
}

// version 返回构建注入的版本、提交、时间和平台。
func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, appversion.Current())
}

// disks 从 SQLite 返回最近持久状态，不因 Web 请求触发硬件扫描。
func (s *Server) disks(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.Disks(r.Context())
	if err != nil {
		writeAPIError(w, 500, "读取硬盘列表失败")
		return
	}
	s.settingsMu.RLock()
	showSerial := s.settings.ShowMaskedSerial
	s.settingsMu.RUnlock()
	if !showSerial {
		for i := range d {
			d[i].MaskedSerial = ""
		}
	}
	writeJSON(w, 200, map[string]any{"items": d})
}

// overview 汇总机械盘状态、今日唤醒和服务健康信息。
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	disks, err := s.store.Disks(r.Context())
	if err != nil {
		writeAPIError(w, 500, "读取总览失败")
		return
	}
	counts := map[event.DiskState]int{}
	mechanical := 0
	lastWake := time.Time{}
	suspect := "暂无证据"
	for _, d := range disks {
		if d.Rotational {
			mechanical++
			counts[d.State]++
		}
	}
	p, _ := s.store.Events(r.Context(), database.EventFilter{Since: startOfLocalDay(time.Now()), Type: string(event.DiskWakeup), Page: 1, PageSize: 200})
	if len(p.Items) > 0 {
		lastWake = p.Items[0].StartedAt
		suspect = firstNonempty(p.Items[0].SuspectApp, p.Items[0].SuspectProcess, p.Items[0].SuspectDocker, "暂无证据")
	}
	s.collectorMu.RLock()
	lastCollector, lastRefresh := s.lastCollector, s.lastRefresh
	s.collectorMu.RUnlock()
	collectorOK := !lastCollector.IsZero() && time.Since(lastCollector) < 2*time.Minute
	dbStatus := "正常"
	if err := s.store.Ping(r.Context()); err != nil {
		dbStatus = "异常"
	}
	writeJSON(w, 200, map[string]any{"mechanicalDisks": mechanical, "activeDisks": counts[event.StateActive] + counts[event.StateIdle], "standbyDisks": counts[event.StateStandby], "unknownDisks": counts[event.StateUnknown], "todayWakeups": p.Total, "lastWakeupAt": nullableTime(lastWake), "suspectedSource": suspect, "collectorHealthy": collectorOK, "databaseStatus": dbStatus, "lastRefreshAt": nullableTime(lastRefresh)})
}

// events 校验筛选条件后返回有上限的倒序事件页。
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	f, err := parseEventFilter(r)
	if err != nil {
		writeAPIError(w, 400, err.Error())
		return
	}
	p, err := s.store.Events(r.Context(), f)
	if err != nil {
		writeAPIError(w, 500, "读取事件失败")
		return
	}
	writeJSON(w, 200, p)
}

// exportCSV 每次读取 200 条事件并立即写响应，避免一次加载全部历史。
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	f, err := parseEventFilter(r)
	if err != nil {
		writeAPIError(w, 400, err.Error())
		return
	}
	f.PageSize = 200
	f.Page = 1
	if u, ok := auth.UserFromContext(r.Context()); ok {
		_ = s.store.LogOperation(r.Context(), u.UID, u.Username, "export_csv", "started", "流式导出事件")
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="disk-wakeup-events.csv"`)
	_, _ = io.WriteString(w, "\xEF\xBB\xBF")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"事件ID", "硬盘", "事件类型", "原状态", "新状态", "开始时间", "读取增量", "写入增量", "疑似进程", "疑似fnOS应用", "疑似容器", "判断依据", "可信度", "备注"})
	for {
		p, err := s.store.Events(r.Context(), f)
		if err != nil {
			return
		}
		for _, v := range p.Items {
			_ = cw.Write([]string{strconv.FormatInt(v.ID, 10), v.Device, string(v.Type), string(v.FromState), string(v.ToState), v.StartedAt.Local().Format(time.RFC3339), strconv.FormatUint(v.ReadDelta, 10), strconv.FormatUint(v.WriteDelta, 10), v.SuspectProcess, v.SuspectApp, v.SuspectDocker, v.Reason, v.Confidence, v.Note})
		}
		cw.Flush()
		if len(p.Items) < f.PageSize {
			return
		}
		f.Page++
	}
}

// getSettings 返回当前已校验设置副本。
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	s.settingsMu.RLock()
	v := s.settings
	s.settingsMu.RUnlock()
	writeJSON(w, 200, v)
}

// putSettings 限制请求体、校验全部范围并原子保存，失败时不返回成功。
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var v config.Settings
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeAPIError(w, 400, "设置 JSON 无效")
		return
	}
	if err := v.Validate(); err != nil {
		writeAPIError(w, 422, err.Error())
		return
	}
	if err := config.Save(s.settingsPath, v); err != nil {
		writeAPIError(w, 500, "设置文件保存失败")
		return
	}
	if err := s.store.SaveSettings(r.Context(), v); err != nil {
		writeAPIError(w, 500, "设置数据库保存失败")
		return
	}
	s.settingsMu.Lock()
	s.settings = v
	s.settingsMu.Unlock()
	s.machine.UpdateConfirmations(v.StateConfirmations)
	if u, ok := auth.UserFromContext(r.Context()); ok {
		_ = s.store.LogOperation(r.Context(), u.UID, u.Username, "update_settings", "success", "设置已原子保存")
	}
	writeJSON(w, 200, map[string]any{"ok": true, "settings": v})
}

// refresh 仅记录安全刷新请求，等待既定采样周期而不立即执行硬件查询。
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	s.collectorMu.Lock()
	s.lastRefresh = time.Now()
	s.collectorMu.Unlock()
	if u, ok := auth.UserFromContext(r.Context()); ok {
		_ = s.store.LogOperation(r.Context(), u.UID, u.Username, "request_refresh", "accepted", "等待下一安全采样周期")
	}
	writeJSON(w, 202, map[string]any{"ok": true, "message": "已请求刷新；Collector 将在下一安全采样周期上报"})
}

// diagnostics 返回已经脱敏的页面预览报告。
func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	report := s.report(r.Context())
	writeJSON(w, 200, report)
}

// diagnosticsText 流式输出脱敏文本报告。
func (s *Server) diagnosticsText(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fn-disk-wakeup-diagnostic.txt"`)
	_, _ = io.WriteString(w, s.report(r.Context()).Text())
}

// diagnosticsJSON 输出脱敏 JSON 报告。
func (s *Server) diagnosticsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fn-disk-wakeup-diagnostic.json"`)
	v, _ := s.report(r.Context()).JSON()
	_, _ = io.WriteString(w, v)
}

// report 从白名单数据源组合诊断信息，不读取任意文件或完整环境变量。
func (s *Server) report(ctx context.Context) diagnose.Report {
	d, _ := s.store.Disks(ctx)
	size, _ := s.store.Size()
	schema, _ := s.store.SchemaVersion(ctx)
	s.settingsMu.RLock()
	settings := s.settings
	s.settingsMu.RUnlock()
	if !settings.ShowMaskedSerial {
		for i := range d {
			d[i].MaskedSerial = ""
		}
	}
	s.collectorMu.RLock()
	ok := !s.lastCollector.IsZero() && time.Since(s.lastCollector) < 2*time.Minute
	s.collectorMu.RUnlock()
	return diagnose.New(settings, d, size, schema, ok, s.gatewaySocket, s.collectorSocket)
}

// mutation 对管理员修改操作按 UID 限流，阻止重复提交。
func (s *Server) mutation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFromContext(r.Context())
		if !s.mutations.Allow(u.UID) {
			writeAPIError(w, 429, "操作过于频繁，请稍后再试")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spa 仅从构建后的 Web 目录提供静态资源，并安全回退到 index.html。
func (s *Server) spa() http.Handler {
	root := os.DirFS(s.webDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, GatewayPrefix)
		rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
		if rel == "" || rel == "." {
			rel = "index.html"
		}
		if strings.Contains(rel, "..") {
			http.NotFound(w, r)
			return
		}
		b, err := fs.ReadFile(root, rel)
		if err != nil {
			b, err = fs.ReadFile(root, "index.html")
		}
		if err != nil {
			writeAPIError(w, 503, "前端资源未安装")
			return
		}
		if typ := mime.TypeByExtension(path.Ext(rel)); typ != "" {
			w.Header().Set("Content-Type", typ)
		}
		_, _ = w.Write(b)
	})
}

// parseEventFilter 把查询参数限制在 24h、7d、30d 和受控分页筛选范围。
func parseEventFilter(r *http.Request) (database.EventFilter, error) {
	q := r.URL.Query()
	f := database.EventFilter{DiskID: q.Get("diskId"), Type: q.Get("type"), Confidence: q.Get("confidence"), Source: q.Get("source")}
	f.Page, _ = strconv.Atoi(q.Get("page"))
	f.PageSize, _ = strconv.Atoi(q.Get("pageSize"))
	switch q.Get("range") {
	case "", "24h":
		f.Since = time.Now().Add(-24 * time.Hour)
	case "7d":
		f.Since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		f.Since = time.Now().Add(-30 * 24 * time.Hour)
	default:
		return f, errors.New("时间范围必须为 24h、7d 或 30d")
	}
	return f, nil
}

// securityHeaders 设置 CSP、no-sniff、无缓存等统一响应安全头。
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'self'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// writeJSON 以统一 UTF-8 JSON 格式写入状态码和响应体。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeAPIError 返回不包含 SQL、路径或堆栈的错误结构。
func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "status": status})
}

// startOfLocalDay 计算用户当前时区的当天零点。
func startOfLocalDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// nullableTime 将零时间转换为 JSON null。
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// firstNonempty 返回首个可展示的疑似来源字段。
func firstNonempty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// findDisk 从状态机快照中按稳定 ID 查找磁盘。
func findDisk(disks []event.Disk, id string) (event.Disk, bool) {
	for _, d := range disks {
		if d.ID == id {
			return d, true
		}
	}
	return event.Disk{}, false
}

// samePersistentDisk 比较需要持久化的字段，忽略高频内存计数。
func samePersistentDisk(a, b event.Disk) bool {
	return a.Device == b.Device && a.Model == b.Model && a.MaskedSerial == b.MaskedSerial && a.CapacityBytes == b.CapacityBytes && a.BusType == b.BusType && a.Rotational == b.Rotational && a.State == b.State && a.PreviousState == b.PreviousState && a.DetectionMethod == b.DetectionMethod && a.CapabilitySupported == b.CapabilitySupported && a.Present == b.Present
}

// capabilityDetail 明确说明能力关闭、不适用或待真机验证状态。
func capabilityDetail(d event.Disk) string {
	if !d.Rotational {
		return "非机械介质，休眠状态不适用"
	}
	if !d.CapabilitySupported {
		return "能力默认关闭或当前平台无法无损探测"
	}
	return "固定参数状态查询；控制器行为仍需真机验证"
}

// filterLowConfidence 在管理员关闭低可信度记录时保留中高证据和应用自身标记。
func filterLowConfidence(values []event.Evidence) []event.Evidence {
	filtered := make([]event.Evidence, 0, len(values))
	for _, value := range values {
		if value.Confidence != "低" || value.SelfActivity {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

// limiter 保存每个管理员最近一次修改时间，内存大小受管理员 UID 数量限制。
type limiter struct {
	mu   sync.Mutex
	last map[string]time.Time
	gap  time.Duration
}

// newLimiter 创建固定最小间隔的修改限流器。
func newLimiter(gap time.Duration) *limiter { return &limiter{last: map[string]time.Time{}, gap: gap} }

// Allow 在锁保护下判断指定 UID 是否已超过最小操作间隔。
func (l *limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if prev := l.last[key]; !prev.IsZero() && now.Sub(prev) < l.gap {
		return false
	}
	l.last[key] = now
	return true
}

var _ = fmt.Sprintf
