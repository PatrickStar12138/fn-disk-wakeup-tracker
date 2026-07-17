package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
)

const schemaVersion = 1

// Store 封装单连接 SQLite，序列化所有写入并使用有限 busy timeout。
type Store struct {
	db   *sql.DB
	path string
}

// EventFilter 表示分页事件查询的受控筛选条件。
type EventFilter struct {
	// Since 是查询开始时间。
	Since time.Time
	// DiskID 是可选稳定物理盘标识。
	DiskID string
	// Type 是可选固定事件类型。
	Type string
	// Confidence 是可选高、中、低可信度。
	Confidence string
	// Source 是可选疑似来源模糊匹配文本。
	Source string
	// Page 从 1 开始。
	Page int
	// PageSize 最大为 200。
	PageSize int
}

// EventPage 返回有限大小的事件页和总数。
type EventPage struct {
	// Items 是当前页事件。
	Items []event.Record `json:"items"`
	// Page 是当前页码。
	Page int `json:"page"`
	// Size 是实际页大小上限。
	Size int `json:"pageSize"`
	// Total 是筛选后的总条数。
	Total int `json:"total"`
}

// Open 创建或打开数据库，设置 WAL、外键和有限锁等待后执行迁移。
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, pragma := range []string{"PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=NORMAL", "PRAGMA busy_timeout=2000", "PRAGMA wal_autocheckpoint=1000"} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite %s: %w", pragma, err)
		}
	}
	s := &Store{db: db, path: path}
	if err := s.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭单连接数据库并等待当前数据库调用返回。
func (s *Store) Close() error { return s.db.Close() }

// Migrate 在单个事务中重复执行兼容迁移，失败时自动回滚且不重建数据库。
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS disks (
            id TEXT PRIMARY KEY, device TEXT NOT NULL UNIQUE, model TEXT NOT NULL DEFAULT '', masked_serial TEXT NOT NULL DEFAULT '',
            capacity_bytes INTEGER NOT NULL DEFAULT 0, bus_type TEXT NOT NULL DEFAULT '', rotational INTEGER NOT NULL DEFAULT 0,
            state TEXT NOT NULL, previous_state TEXT NOT NULL, last_state_change TEXT NOT NULL, today_wakeups INTEGER NOT NULL DEFAULT 0,
            last_activity TEXT, detection_method TEXT NOT NULL DEFAULT '', capability_supported INTEGER NOT NULL DEFAULT 0,
            present INTEGER NOT NULL DEFAULT 1, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS disk_capabilities (
            disk_id TEXT NOT NULL, capability TEXT NOT NULL, supported INTEGER NOT NULL, method TEXT NOT NULL DEFAULT '',
            wake_risk TEXT NOT NULL DEFAULT 'unknown', detail TEXT NOT NULL DEFAULT '', checked_at TEXT NOT NULL,
            PRIMARY KEY (disk_id, capability), FOREIGN KEY (disk_id) REFERENCES disks(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS disk_state_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT, disk_id TEXT, event_type TEXT NOT NULL, from_state TEXT NOT NULL,
            to_state TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT, duration_ms INTEGER NOT NULL DEFAULT 0,
            read_delta INTEGER NOT NULL DEFAULT 0, write_delta INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '',
            confidence TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '', FOREIGN KEY (disk_id) REFERENCES disks(id) ON DELETE SET NULL)`,
		`CREATE TABLE IF NOT EXISTS wake_events (
            event_id INTEGER PRIMARY KEY, suspect_process TEXT NOT NULL DEFAULT '', suspect_fnos_app TEXT NOT NULL DEFAULT '',
            suspect_container TEXT NOT NULL DEFAULT '', FOREIGN KEY (event_id) REFERENCES disk_state_events(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS attribution_evidence (
            id INTEGER PRIMARY KEY AUTOINCREMENT, event_id INTEGER NOT NULL, pid INTEGER NOT NULL DEFAULT 0,
            process TEXT NOT NULL DEFAULT '', executable TEXT NOT NULL DEFAULT '', command_line TEXT NOT NULL DEFAULT '',
            cgroup TEXT NOT NULL DEFAULT '', fnos_app TEXT NOT NULL DEFAULT '', container TEXT NOT NULL DEFAULT '',
            read_delta INTEGER NOT NULL DEFAULT 0, write_delta INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '',
            confidence TEXT NOT NULL DEFAULT '', self_activity INTEGER NOT NULL DEFAULT 0,
            FOREIGN KEY (event_id) REFERENCES disk_state_events(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS daily_statistics (
            day TEXT NOT NULL, disk_id TEXT NOT NULL, wakeups INTEGER NOT NULL DEFAULT 0, active_seconds INTEGER NOT NULL DEFAULT 0,
            read_bytes INTEGER NOT NULL DEFAULT 0, write_bytes INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (day, disk_id),
            FOREIGN KEY (disk_id) REFERENCES disks(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value_json TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS diagnostic_runs (
            id INTEGER PRIMARY KEY AUTOINCREMENT, created_at TEXT NOT NULL, format_version INTEGER NOT NULL, status TEXT NOT NULL,
            summary_json TEXT NOT NULL DEFAULT '{}')`,
		`CREATE TABLE IF NOT EXISTS operation_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT, created_at TEXT NOT NULL, actor_uid TEXT NOT NULL DEFAULT '', actor_name TEXT NOT NULL DEFAULT '',
            operation TEXT NOT NULL, result TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '')`,
		`CREATE INDEX IF NOT EXISTS idx_state_events_started ON disk_state_events(started_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_state_events_disk_started ON disk_state_events(disk_id, started_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_state_events_type ON disk_state_events(event_type, started_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_evidence_source ON attribution_evidence(process, fnos_app, container)`,
		`CREATE INDEX IF NOT EXISTS idx_operation_logs_created ON operation_logs(created_at DESC)`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ','now'))`,
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration %d: %w", schemaVersion, err)
		}
	}
	return tx.Commit()
}

// UpsertDisk 仅在设备首次出现、能力或状态变化时由上层调用，避免按采样频率写库。
func (s *Store) UpsertDisk(ctx context.Context, d event.Disk) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO disks
        (id,device,model,masked_serial,capacity_bytes,bus_type,rotational,state,previous_state,last_state_change,today_wakeups,last_activity,detection_method,capability_supported,present,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(id) DO UPDATE SET device=excluded.device, model=excluded.model, masked_serial=excluded.masked_serial,
        capacity_bytes=excluded.capacity_bytes, bus_type=excluded.bus_type, rotational=excluded.rotational, state=excluded.state,
        previous_state=excluded.previous_state, last_state_change=excluded.last_state_change, today_wakeups=excluded.today_wakeups,
        last_activity=excluded.last_activity, detection_method=excluded.detection_method, capability_supported=excluded.capability_supported,
        present=excluded.present, updated_at=excluded.updated_at`,
		d.ID, d.Device, d.Model, d.MaskedSerial, d.CapacityBytes, d.BusType, d.Rotational, d.State, d.PreviousState,
		timeText(d.LastStateChange), d.TodayWakeups, nullableTime(d.LastActivity), d.DetectionMethod, d.CapabilitySupported, d.Present, timeText(time.Now()))
	return err
}

// UpsertPowerCapability 持久化状态探测能力及其可能唤醒设备的风险说明。
func (s *Store) UpsertPowerCapability(ctx context.Context, d event.Disk, detail string) error {
	wakeRisk := "unknown"
	if !d.Rotational {
		wakeRisk = "not_applicable"
	} else if d.DetectionMethod == "hdparm-C" {
		wakeRisk = "low_requires_hardware_validation"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO disk_capabilities(disk_id,capability,supported,method,wake_risk,detail,checked_at)
        VALUES(?,?,?,?,?,?,?) ON CONFLICT(disk_id,capability) DO UPDATE SET supported=excluded.supported,method=excluded.method,
        wake_risk=excluded.wake_risk,detail=excluded.detail,checked_at=excluded.checked_at`, d.ID, "power_state", d.CapabilitySupported, d.DetectionMethod, wakeRisk, detail, timeText(time.Now()))
	return err
}

// InsertEvent 在一个事务中写入事件、唤醒摘要和全部证据，防止半条事件。
func (s *Store) InsertEvent(ctx context.Context, r *event.Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO disk_state_events
        (disk_id,event_type,from_state,to_state,started_at,ended_at,duration_ms,read_delta,write_delta,reason,confidence,note)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, nullableString(r.DiskID), r.Type, r.FromState, r.ToState, timeText(r.StartedAt), nullableTimePtr(r.EndedAt),
		r.DurationMS, r.ReadDelta, r.WriteDelta, r.Reason, r.Confidence, r.Note)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	r.ID = id
	if r.Type == event.DiskWakeup {
		if _, err := tx.ExecContext(ctx, `INSERT INTO wake_events(event_id,suspect_process,suspect_fnos_app,suspect_container) VALUES(?,?,?,?)`, id, r.SuspectProcess, r.SuspectApp, r.SuspectDocker); err != nil {
			return err
		}
	}
	for _, e := range r.Evidence {
		if _, err := tx.ExecContext(ctx, `INSERT INTO attribution_evidence
            (event_id,pid,process,executable,command_line,cgroup,fnos_app,container,read_delta,write_delta,reason,confidence,self_activity)
            VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, e.PID, e.Process, e.Executable, e.CommandLine, e.Cgroup, e.FnosApp, e.Container,
			e.ReadDelta, e.WriteDelta, e.Reason, e.Confidence, e.SelfActivity); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Disks 返回最近持久化的物理盘状态，不触发任何硬件访问。
func (s *Store) Disks(ctx context.Context) ([]event.Disk, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,device,model,masked_serial,capacity_bytes,bus_type,rotational,state,previous_state,
        last_state_change,(SELECT COUNT(*) FROM disk_state_events e WHERE e.disk_id=disks.id AND e.event_type='disk_wakeup' AND e.started_at>=?),
        COALESCE(last_activity,''),detection_method,capability_supported,present FROM disks ORDER BY device`, timeText(startOfDay(time.Now())))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []event.Disk
	for rows.Next() {
		var d event.Disk
		var rotational, supported, present int
		var changed, activity string
		if err := rows.Scan(&d.ID, &d.Device, &d.Model, &d.MaskedSerial, &d.CapacityBytes, &d.BusType, &rotational, &d.State, &d.PreviousState,
			&changed, &d.TodayWakeups, &activity, &d.DetectionMethod, &supported, &present); err != nil {
			return nil, err
		}
		d.Rotational, d.CapabilitySupported, d.Present = rotational != 0, supported != 0, present != 0
		d.LastStateChange, _ = time.Parse(time.RFC3339Nano, changed)
		d.LastActivity, _ = time.Parse(time.RFC3339Nano, activity)
		out = append(out, d)
	}
	return out, rows.Err()
}

// Events 使用参数化 SQL 和最大 200 条页大小查询事件，避免无界内存占用。
func (s *Store) Events(ctx context.Context, f EventFilter) (EventPage, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 200 {
		f.PageSize = 200
	}
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	if !f.Since.IsZero() {
		where = append(where, "e.started_at>=?")
		args = append(args, timeText(f.Since))
	}
	if f.DiskID != "" {
		where = append(where, "e.disk_id=?")
		args = append(args, f.DiskID)
	}
	if f.Type != "" {
		where = append(where, "e.event_type=?")
		args = append(args, f.Type)
	}
	if f.Confidence != "" {
		where = append(where, "e.confidence=?")
		args = append(args, f.Confidence)
	}
	if f.Source != "" {
		where = append(where, "(w.suspect_process LIKE ? OR w.suspect_fnos_app LIKE ? OR w.suspect_container LIKE ?)")
		q := "%" + f.Source + "%"
		args = append(args, q, q, q)
	}
	base := ` FROM disk_state_events e LEFT JOIN wake_events w ON w.event_id=e.id WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+base, args...).Scan(&total); err != nil {
		return EventPage{}, err
	}
	queryArgs := append(append([]any{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,COALESCE(e.disk_id,''),COALESCE((SELECT device FROM disks d WHERE d.id=e.disk_id),''),
        e.event_type,e.from_state,e.to_state,e.started_at,COALESCE(e.ended_at,''),e.duration_ms,e.read_delta,e.write_delta,
        COALESCE(w.suspect_process,''),COALESCE(w.suspect_fnos_app,''),COALESCE(w.suspect_container,''),e.reason,e.confidence,e.note`+base+` ORDER BY e.started_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return EventPage{}, err
	}
	defer rows.Close()
	page := EventPage{Page: f.Page, Size: f.PageSize, Total: total, Items: make([]event.Record, 0)}
	for rows.Next() {
		var r event.Record
		var started, ended string
		if err := rows.Scan(&r.ID, &r.DiskID, &r.Device, &r.Type, &r.FromState, &r.ToState, &started, &ended, &r.DurationMS, &r.ReadDelta, &r.WriteDelta, &r.SuspectProcess, &r.SuspectApp, &r.SuspectDocker, &r.Reason, &r.Confidence, &r.Note); err != nil {
			return EventPage{}, err
		}
		r.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		if ended != "" {
			v, _ := time.Parse(time.RFC3339Nano, ended)
			r.EndedAt = &v
		}
		page.Items = append(page.Items, r)
	}
	return page, rows.Err()
}

// SaveSettings 保存已校验设置的数据库快照，便于诊断和迁移。
func (s *Store) SaveSettings(ctx context.Context, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at) VALUES('settings',?,?)
        ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, updated_at=excluded.updated_at`, string(b), timeText(time.Now()))
	return err
}

// Cleanup 以每批 500 条低频删除过期数据，并在超过体积上限时有界清理旧事件。
func (s *Store) Cleanup(ctx context.Context, retentionDays, maxMB int) (int64, error) {
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	var deleted int64
	for i := 0; i < 100; i++ {
		res, err := s.db.ExecContext(ctx, `DELETE FROM disk_state_events WHERE id IN (SELECT id FROM disk_state_events WHERE started_at < ? ORDER BY id LIMIT 500)`, timeText(cutoff))
		if err != nil {
			return deleted, err
		}
		n, _ := res.RowsAffected()
		deleted += n
		if n < 500 {
			break
		}
	}
	for _, table := range []string{"operation_logs", "diagnostic_runs"} {
		query := fmt.Sprintf("DELETE FROM %s WHERE id IN (SELECT id FROM %s WHERE created_at < ? ORDER BY id LIMIT 500)", table, table)
		for i := 0; i < 100; i++ {
			res, err := s.db.ExecContext(ctx, query, timeText(cutoff))
			if err != nil {
				return deleted, err
			}
			n, _ := res.RowsAffected()
			if n < 500 {
				break
			}
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM daily_statistics WHERE day < ?`, cutoff.UTC().Format("2006-01-02")); err != nil {
		return deleted, err
	}
	if size, _ := s.Size(); size > int64(maxMB)*1024*1024 {
		for i := 0; i < 100 && size > int64(maxMB)*1024*1024; i++ {
			res, err := s.db.ExecContext(ctx, `DELETE FROM disk_state_events WHERE id IN (SELECT id FROM disk_state_events ORDER BY started_at LIMIT 500)`)
			if err != nil {
				return deleted, err
			}
			n, _ := res.RowsAffected()
			deleted += n
			if n == 0 {
				break
			}
			size, _ = s.Size()
		}
	}
	return deleted, nil
}

// AggregateDaily 以单条聚合 SQL 更新指定日期统计，只由每日后台任务调用。
func (s *Store) AggregateDaily(ctx context.Context, at time.Time) error {
	day := at.UTC().Format("2006-01-02")
	_, err := s.db.ExecContext(ctx, `INSERT INTO daily_statistics(day,disk_id,wakeups,read_bytes,write_bytes)
        SELECT ?,disk_id,SUM(CASE WHEN event_type='disk_wakeup' THEN 1 ELSE 0 END),SUM(read_delta),SUM(write_delta)
        FROM disk_state_events WHERE disk_id IS NOT NULL AND substr(started_at,1,10)=? GROUP BY disk_id
        ON CONFLICT(day,disk_id) DO UPDATE SET wakeups=excluded.wakeups,read_bytes=excluded.read_bytes,write_bytes=excluded.write_bytes`, day, day)
	return err
}

// LogOperation 记录管理员修改和导出摘要，并限制详情长度防止日志膨胀。
func (s *Store) LogOperation(ctx context.Context, uid, username, operation, result, detail string) error {
	if len(detail) > 512 {
		detail = detail[:512]
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO operation_logs(created_at,actor_uid,actor_name,operation,result,detail) VALUES(?,?,?,?,?,?)`, timeText(time.Now()), uid, username, operation, result, detail)
	return err
}

// Size 返回 SQLite 主文件字节数，不读取被监测硬盘上的用户文件。
func (s *Store) Size() (int64, error) {
	st, err := os.Stat(s.path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// Ping 在请求上下文内检查数据库连接状态。
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// SchemaVersion 返回已成功应用的最高迁移版本。
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&v)
	return v, err
}

// timeText 将时间统一转换为 UTC RFC3339Nano；零值使用当前时间。
func timeText(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// nullableTime 把零时间转换为 SQL NULL。
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return timeText(t)
}

// nullableTimePtr 把空时间指针转换为 SQL NULL。
func nullableTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return timeText(*t)
}

// nullableString 把空字符串转换为 SQL NULL，供可选外键使用。
func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// startOfDay 返回当前本地日期零点，随后由 timeText 转为可比较 UTC 时间。
func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// ErrClosed 表示调用方尝试使用已关闭数据库。
var ErrClosed = errors.New("database is closed")
