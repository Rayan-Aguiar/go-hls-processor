package service

import (
    "context"
    "database/sql"
    "errors"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/rayan-aguiar/video-processor/internal/db"
)

type mockPublisher struct {
    publishFn func(ctx context.Context, jobID string) error
}

func (m *mockPublisher) PublishJob(ctx context.Context, jobID string) error {
    if m.publishFn != nil {
        return m.publishFn(ctx, jobID)
    }
    return nil
}

func TestUploadAndValidateFileSuccess(t *testing.T) {
    tmpDir := t.TempDir()
    conn := newTestDB(t, filepath.Join(tmpDir, "app.db"))

    var publishedJobID string
    publisher := &mockPublisher{
        publishFn: func(_ context.Context, jobID string) error {
            publishedJobID = jobID
            return nil
        },
    }

    service := New(filepath.Join(tmpDir, "uploads"), conn, publisher)

    input := UploadFileInput{
        Filename: "video.mp4",
        FileSize: int64(len("fake-video-content")),
        Reader:   strings.NewReader("fake-video-content"),
    }

    output, err := service.UploadAndValidateFile(context.Background(), input)
    if err != nil {
        t.Fatalf("expected nil error, got %v", err)
    }

    if output.JobID == "" {
        t.Fatal("expected job id to be generated")
    }

    if output.Status != "pending" {
        t.Fatalf("expected status pending, got %s", output.Status)
    }

    if publishedJobID != output.JobID {
        t.Fatalf("expected published job id %s, got %s", output.JobID, publishedJobID)
    }

    content, err := os.ReadFile(output.InputPath)
    if err != nil {
        t.Fatalf("expected saved file to exist, got %v", err)
    }

    if string(content) != "fake-video-content" {
        t.Fatalf("expected file content to match input, got %q", string(content))
    }

    job, err := db.GetJobByID(context.Background(), conn, output.JobID)
    if err != nil {
        t.Fatalf("expected job in database, got %v", err)
    }

    if job.InputPath != output.InputPath {
        t.Fatalf("expected input path %s, got %s", output.InputPath, job.InputPath)
    }

    if job.Status != "pending" {
        t.Fatalf("expected db status pending, got %s", job.Status)
    }

    if job.OutputDir.Valid {
        t.Fatal("expected output_dir to be null for pending job")
    }
}

func TestUploadAndValidateFileCleansUpOnPersistError(t *testing.T) {
    tmpDir := t.TempDir()
    conn := newTestDB(t, filepath.Join(tmpDir, "app.db"))

    publisher := &mockPublisher{}
    service := New(filepath.Join(tmpDir, "uploads"), conn, publisher)

    if err := conn.Close(); err != nil {
        t.Fatalf("failed to close db: %v", err)
    }

    _, err := service.UploadAndValidateFile(context.Background(), UploadFileInput{
        Filename: "video.mp4",
        FileSize: int64(len("fake-video-content")),
        Reader:   strings.NewReader("fake-video-content"),
    })
    if err == nil {
        t.Fatal("expected error when db is closed, got nil")
    }

    entries, readErr := os.ReadDir(filepath.Join(tmpDir, "uploads"))
    if readErr != nil {
        if os.IsNotExist(readErr) {
            return
        }
        t.Fatalf("expected uploads dir to be readable, got %v", readErr)
    }

    if len(entries) != 0 {
        t.Fatalf("expected uploads dir to be empty after cleanup, found %d entries", len(entries))
    }
}

func TestUploadAndValidateFileRollsBackWhenPublishFails(t *testing.T) {
    tmpDir := t.TempDir()
    conn := newTestDB(t, filepath.Join(tmpDir, "app.db"))

    publisher := &mockPublisher{
        publishFn: func(_ context.Context, _ string) error {
            return errors.New("redis unavailable")
        },
    }

    service := New(filepath.Join(tmpDir, "uploads"), conn, publisher)

    _, err := service.UploadAndValidateFile(context.Background(), UploadFileInput{
        Filename: "video.mp4",
        FileSize: int64(len("fake-video-content")),
        Reader:   strings.NewReader("fake-video-content"),
    })
    if err == nil {
        t.Fatal("expected publish error, got nil")
    }

    entries, readErr := os.ReadDir(filepath.Join(tmpDir, "uploads"))
    if readErr != nil {
        if os.IsNotExist(readErr) {
            return
        }
        t.Fatalf("expected uploads dir to be readable, got %v", readErr)
    }

    if len(entries) != 0 {
        t.Fatalf("expected uploads dir to be empty after rollback, found %d entries", len(entries))
    }

    rows, queryErr := conn.Query(`SELECT COUNT(*) FROM jobs`)
    if queryErr != nil {
        t.Fatalf("failed to query jobs count: %v", queryErr)
    }
    defer rows.Close()

    var count int
    if rows.Next() {
        if scanErr := rows.Scan(&count); scanErr != nil {
            t.Fatalf("failed to scan jobs count: %v", scanErr)
        }
    }

    if count != 0 {
        t.Fatalf("expected zero jobs after rollback, found %d", count)
    }
}

func newTestDB(t *testing.T, dbPath string) *sql.DB {
    t.Helper()

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