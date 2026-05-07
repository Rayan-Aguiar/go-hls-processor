package ffmpeg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHLSConverterConvert(t *testing.T) {
	runner := &stubRunner{}
	converter := NewHLSConverter(runner)
	outputDir := t.TempDir()

	output, err := converter.Convert(context.Background(), "input.mp4", outputDir)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(runner.calls) != len(Qualities) {
		t.Fatalf("expected %d ffmpeg calls, got %d", len(Qualities), len(runner.calls))
	}

	if _, err := os.Stat(output.MasterPlaylist); err != nil {
		t.Fatalf("expected master playlist to exist, got %v", err)
	}

	if len(output.Qualities) != len(Qualities) {
		t.Fatalf("expected %d quality playlists, got %d", len(Qualities), len(output.Qualities))
	}

	firstCall := strings.Join(runner.calls[0], " ")
	if !strings.Contains(firstCall, "scale=640:360") {
		t.Fatalf("expected first call to include 360p scale, got %s", firstCall)
	}

	lastCall := strings.Join(runner.calls[len(runner.calls)-1], " ")
	if !strings.Contains(lastCall, "scale=1920:1080") {
		t.Fatalf("expected last call to include 1080p scale, got %s", lastCall)
	}

	content, err := os.ReadFile(output.MasterPlaylist)
	if err != nil {
		t.Fatalf("expected to read master playlist, got %v", err)
	}

	master := string(content)
	if !strings.Contains(master, "360p/index.m3u8") {
		t.Fatalf("expected master playlist to reference 360p variant, got %s", master)
	}
	if !strings.Contains(master, "1080p/index.m3u8") {
		t.Fatalf("expected master playlist to reference 1080p variant, got %s", master)
	}

	for _, q := range Qualities {
		wantPath := filepath.Join(outputDir, q.Name, "index.m3u8")
		if output.Qualities[q.Name] != wantPath {
			t.Fatalf("expected quality path %s, got %s", wantPath, output.Qualities[q.Name])
		}
	}
}

func TestHLSConverterConvertPropagatesRunnerError(t *testing.T) {
	runner := &stubRunner{err: errors.New("runner failed")}
	converter := NewHLSConverter(runner)

	_, err := converter.Convert(context.Background(), "input.mp4", t.TempDir())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "falha ao converter qualidade") {
		t.Fatalf("expected conversion error, got %v", err)
	}
}

type stubRunner struct {
	calls [][]string
	err   error
}

func (s *stubRunner) Run(_ context.Context, args ...string) error {
	call := append([]string(nil), args...)
	s.calls = append(s.calls, call)
	return s.err
}
