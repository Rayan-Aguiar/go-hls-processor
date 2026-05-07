package ffmpeg

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestThumbnailGeneratorGenerate(t *testing.T) {
	runner := &stubRunner{}
	generator := NewThumbnailGenerator(runner)
	outputDir := t.TempDir()

	thumbnailPath, err := generator.Generate(context.Background(), "input.mp4", outputDir)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	wantPath := filepath.Join(outputDir, "thumbnail.jpg")
	if thumbnailPath != wantPath {
		t.Fatalf("expected thumbnail path %s, got %s", wantPath, thumbnailPath)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 ffmpeg call, got %d", len(runner.calls))
	}

	call := strings.Join(runner.calls[0], " ")
	if !strings.Contains(call, "-vframes 1") {
		t.Fatalf("expected thumbnail command to request one frame, got %s", call)
	}
	if !strings.Contains(call, wantPath) {
		t.Fatalf("expected thumbnail command to include output path, got %s", call)
	}
}

func TestThumbnailGeneratorGeneratePropagatesRunnerError(t *testing.T) {
	runner := &stubRunner{err: errors.New("runner failed")}
	generator := NewThumbnailGenerator(runner)

	_, err := generator.Generate(context.Background(), "input.mp4", t.TempDir())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "falha ao gerar thumbnail") {
		t.Fatalf("expected thumbnail error, got %v", err)
	}
}
