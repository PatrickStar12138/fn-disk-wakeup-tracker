package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/event"
)

// TestMigrationIsRepeatableAndPagination 验证迁移可重入且分页不会一次返回全部事件。
func TestMigrationIsRepeatableAndPagination(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if v, err := s.SchemaVersion(ctx); err != nil || v != 1 {
		t.Fatalf("schema=%d err=%v", v, err)
	}
	d := event.Disk{ID: "disk-a", Device: "sda", State: event.StateActive, PreviousState: event.StateUnknown, Present: true, LastStateChange: time.Now()}
	if err := s.UpsertDisk(ctx, d); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		r := event.Record{DiskID: d.ID, Type: event.DiskActivity, FromState: event.StateIdle, ToState: event.StateActive, StartedAt: time.Now().Add(time.Duration(i) * time.Second)}
		if err := s.InsertEvent(ctx, &r); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.Events(ctx, EventFilter{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 3 || len(p.Items) != 1 {
		t.Fatalf("page=%#v", p)
	}
}

// TestWriteFailureAfterClose 验证 SQLite 写入失败会返回错误而不是假装成功。
func TestWriteFailureAfterClose(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	r := event.Record{Type: event.CollectorOffline, StartedAt: time.Now()}
	if err := s.InsertEvent(context.Background(), &r); err == nil {
		t.Fatal("expected write failure")
	}
}

// TestCleanupRetention 验证过期事件按保留策略被删除。
func TestCleanupRetention(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	r := event.Record{Type: event.CollectorOffline, StartedAt: time.Now().Add(-48 * time.Hour)}
	if err := s.InsertEvent(ctx, &r); err != nil {
		t.Fatal(err)
	}
	n, err := s.Cleanup(ctx, 1, 200)
	if err != nil || n != 1 {
		t.Fatalf("deleted=%d err=%v", n, err)
	}
}

// TestBusyLockIsBounded 验证外部写锁存在时 SQLite 在 busy timeout 后返回错误而非无限等待。
func TestBusyLockIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "busy.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	locker, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer locker.Close()
	if _, err := locker.Exec("PRAGMA busy_timeout=50"); err != nil {
		t.Fatal(err)
	}
	if _, err := locker.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer locker.Exec("ROLLBACK") // 测试结束释放外部写锁，避免污染临时数据库。
	started := time.Now()
	err = s.SaveSettings(context.Background(), map[string]bool{"ok": true})
	if err == nil {
		t.Fatal("expected bounded busy error")
	}
	if time.Since(started) > 4*time.Second {
		t.Fatalf("busy wait exceeded bound: %s", time.Since(started))
	}
}
