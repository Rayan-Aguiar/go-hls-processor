package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/ffmpeg"
)

type fakeHLS struct {
    err    error
    called bool
}

func (f *fakeHLS) Convert(ctx context.Context, inputPath, outputDir string) (*ffmpeg.HLSOutput, error) {
    f.called = true
    if f.err != nil {
        return nil, f.err
    }
    return &ffmpeg.HLSOutput{
        MasterPlaylist: filepath.Join(outputDir, "master.m3u8"),
        Qualities: map[string]string{
            "360p": filepath.Join(outputDir, "360p", "index.m3u8"),
        },
    }, nil
}

type fakeThumb struct {
    err    error
    called bool
}

func (f *fakeThumb) Generate(ctx context.Context, inputPath, outputDir string) (string, error) {
    f.called = true
    if f.err != nil {
        return "", f.err
    }
    return filepath.Join(outputDir, "thumbnail.jpg"), nil
}

func newProcessingTestDB(t *testing.T) *sql.DB {
    t.Helper()

    dbPath := filepath.Join(t.TempDir(), "test.db")
    conn, err := db.Open(dbPath)
    if err != nil {
        t.Fatalf("open db: %v", err)
    }

    t.Cleanup(func() { _ = conn.Close() })

    _, err = conn.Exec(`
        CREATE TABLE IF NOT EXISTS jobs (
            id TEXT PRIMARY KEY,
            status TEXT NOT NULL,
            input_path TEXT NOT NULL,
            output_dir TEXT,
            created_at DATETIME NOT NULL,
            updated_at DATETIME
        )
    `)
    if err != nil {
        t.Fatalf("create jobs table: %v", err)
    }

    return conn
}

func seedJob(t *testing.T, conn *sql.DB, jobID string, inputPath string) {
    t.Helper()

    _ = os.MkdirAll(filepath.Dir(inputPath), 0o755)
    if err := os.WriteFile(inputPath, []byte("fake"), 0o644); err != nil {
        t.Fatalf("write input file: %v", err)
    }

    err := db.InsertJob(conn, db.Job{
        ID:        jobID,
        Status:    "pending",
        InputPath: inputPath,
        CreatedAt: time.Now(),
    })
    if err != nil {
        t.Fatalf("insert job: %v", err)
    }
}


func TestProcessJobSuccess(t *testing.T) {
    conn := newProcessingTestDB(t)
    baseDir := t.TempDir()

    jobID := "job-1"
    inputPath := filepath.Join(t.TempDir(), "input.mp4")
    seedJob(t, conn, jobID, inputPath)

    hls := &fakeHLS{}
    thumb := &fakeThumb{}
    svc := NewProcessingService(conn, baseDir, hls, thumb)

    out, err := svc.ProcessJob(context.Background(), jobID)
    if err != nil {
        t.Fatalf("expected nil error, got %v", err)
    }

    if !hls.called {
        t.Fatal("expected hls converter to be called")
    }
    if !thumb.called {
        t.Fatal("expected thumbnail generator to be called")
    }
    if out.JobID != jobID {
        t.Fatalf("expected job id %s, got %s", jobID, out.JobID)
    }

    stored, err := db.GetJobByID(context.Background(), conn, jobID)
    if err != nil {
        t.Fatalf("get job by id: %v", err)
    }
    if stored.Status != "completed" {
        t.Fatalf("expected completed, got %s", stored.Status)
    }
    if !stored.OutputDir.Valid {
        t.Fatal("expected output_dir to be set")
    }
}

func TestProcessJobFailsWhenHLSFails(t *testing.T) {
    conn := newProcessingTestDB(t)
    baseDir := t.TempDir()

    jobID := "job-2"
    inputPath := filepath.Join(t.TempDir(), "input.mp4")
    seedJob(t, conn, jobID, inputPath)

    hls := &fakeHLS{err: errors.New("hls fail")}
    thumb := &fakeThumb{}
    svc := NewProcessingService(conn, baseDir, hls, thumb)

    _, err := svc.ProcessJob(context.Background(), jobID)
    if err == nil {
        t.Fatal("expected error, got nil")
    }

    stored, getErr := db.GetJobByID(context.Background(), conn, jobID)
    if getErr != nil {
        t.Fatalf("get job by id: %v", getErr)
    }
    if stored.Status != "failed" {
        t.Fatalf("expected failed, got %s", stored.Status)
    }
    if thumb.called {
        t.Fatal("thumbnail should not run when hls fails")
    }
}

func TestProcessJobFailsWhenThumbnailFails(t *testing.T) {
    conn := newProcessingTestDB(t)
    baseDir := t.TempDir()

    jobID := "job-3"
    inputPath := filepath.Join(t.TempDir(), "input.mp4")
    seedJob(t, conn, jobID, inputPath)

    hls := &fakeHLS{}
    thumb := &fakeThumb{err: errors.New("thumb fail")}
    svc := NewProcessingService(conn, baseDir, hls, thumb)

    _, err := svc.ProcessJob(context.Background(), jobID)
    if err == nil {
        t.Fatal("expected error, got nil")
    }

    stored, getErr := db.GetJobByID(context.Background(), conn, jobID)
    if getErr != nil {
        t.Fatalf("get job by id: %v", getErr)
    }
    if stored.Status != "failed" {
        t.Fatalf("expected failed, got %s", stored.Status)
    }
}