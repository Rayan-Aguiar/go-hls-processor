package service

import (
    "context"
    "database/sql"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/rayan-aguiar/video-processor/internal/db"
    "github.com/rayan-aguiar/video-processor/internal/queue"
)

type fakeQueueAdapter struct {
    enqueued []queue.JobMessage
}

func (f *fakeQueueAdapter) Enqueue(_ context.Context, _ string, msg queue.JobMessage) error {
    f.enqueued = append(f.enqueued, msg)
    return nil
}

func (f *fakeQueueAdapter) DequeueBlocking(_ context.Context, _ string, _ int) (*queue.JobMessage, error) {
    return nil, nil
}

func (f *fakeQueueAdapter) Len(_ context.Context, _ string) (int64, error) {
    return 0, nil
}

func (f *fakeQueueAdapter) EnqueueWithDelay(_ context.Context, _ string, _ queue.JobMessage, _ time.Duration) error {
    return nil
}

func (f *fakeQueueAdapter) RequeueDue(_ context.Context, _, _ string, _ int64) (int64, error) {
    return 0, nil
}

func (f *fakeQueueAdapter) Close() error {
    return nil
}

func TestRecoveryServiceRecover_RequeuesStuckProcessingJobs(t *testing.T) {
    conn := newRecoveryTestDB(t)

    oldTime := time.Now().Add(-10 * time.Minute)
    insertJobForRecovery(t, conn, "job-stuck", "processing", oldTime, oldTime)

    adapter := &fakeQueueAdapter{}
    svc := NewRecoveryService(conn, adapter, "video:jobs", 2*time.Minute, 100)

    recovered, err := svc.Recover(context.Background())
    if err != nil {
        t.Fatalf("expected nil error, got %v", err)
    }
    if recovered != 1 {
        t.Fatalf("expected recovered=1, got %d", recovered)
    }

    if len(adapter.enqueued) != 1 {
        t.Fatalf("expected 1 enqueued msg, got %d", len(adapter.enqueued))
    }
    if adapter.enqueued[0].JobID != "job-stuck" {
        t.Fatalf("expected job-stuck, got %s", adapter.enqueued[0].JobID)
    }

    job, err := db.GetJobByID(context.Background(), conn, "job-stuck")
    if err != nil {
        t.Fatalf("get job failed: %v", err)
    }
    if job.Status != "pending" {
        t.Fatalf("expected status pending after recovery, got %s", job.Status)
    }
}

func TestRecoveryServiceRecover_DoesNotRequeueFreshProcessingJobs(t *testing.T) {
    conn := newRecoveryTestDB(t)

    now := time.Now()
    insertJobForRecovery(t, conn, "job-fresh", "processing", now, now)

    adapter := &fakeQueueAdapter{}
    svc := NewRecoveryService(conn, adapter, "video:jobs", 2*time.Minute, 100)

    recovered, err := svc.Recover(context.Background())
    if err != nil {
        t.Fatalf("expected nil error, got %v", err)
    }
    if recovered != 0 {
        t.Fatalf("expected recovered=0, got %d", recovered)
    }
    if len(adapter.enqueued) != 0 {
        t.Fatalf("expected no enqueued messages, got %d", len(adapter.enqueued))
    }
}

func newRecoveryTestDB(t *testing.T) *sql.DB {
    t.Helper()

    dbPath := filepath.Join(t.TempDir(), "recovery.db")
    conn, err := db.Open(dbPath)
    if err != nil {
        t.Fatalf("failed to open test db: %v", err)
    }

    t.Cleanup(func() {
        _ = conn.Close()
    })

    _, err = conn.Exec(`CREATE TABLE IF NOT EXISTS jobs (
        id TEXT PRIMARY KEY,
        status TEXT NOT NULL,
        input_path TEXT NOT NULL,
        output_dir TEXT,
        created_at DATETIME NOT NULL,
        updated_at DATETIME
    )`)
    if err != nil {
        t.Fatalf("failed to create jobs table: %v", err)
    }

    return conn
}

func insertJobForRecovery(t *testing.T, conn *sql.DB, id, status string, createdAt, updatedAt time.Time) {
    t.Helper()

    _, err := conn.Exec(`
        INSERT INTO jobs (id, status, input_path, output_dir, created_at, updated_at)
        VALUES (?, ?, ?, NULL, ?, ?)
    `, id, status, filepath.Join(os.TempDir(), id+".mp4"), createdAt, updatedAt)
    if err != nil {
        t.Fatalf("insert job failed: %v", err)
    }
}