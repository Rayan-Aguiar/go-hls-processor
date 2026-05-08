package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type HLSConverter struct {
	runner Runner
}

func NewHLSConverter(runner Runner) *HLSConverter {
	return &HLSConverter{runner: runner}
}

// HLSOutput contém os caminhos gerados após a conversão.
type HLSOutput struct {
	MasterPlaylist string
	Qualities      map[string]string // qualidade -> caminho do playlist
}

// Convert converte o vídeo de entrada para HLS em todas as qualidades
// e gera o master.m3u8 apontando para cada uma delas.
// onQuality é chamado após cada qualidade concluída com (nome, feito, total).
func (c *HLSConverter) Convert(ctx context.Context, inputPath, outputDir string, onQuality func(name string, done, total int)) (*HLSOutput, error) {
	qualityPaths := make(map[string]string)

	for i, q := range Qualities {
		playlistPath, err := c.convertQuality(ctx, inputPath, outputDir, q)
		if err != nil {
			return nil, fmt.Errorf("falha ao converter qualidade %s: %w", q.Name, err)
		}
		qualityPaths[q.Name] = playlistPath
		if onQuality != nil {
			onQuality(q.Name, i+1, len(Qualities))
		}
	}

	masterPath, err := c.writeMasterPlaylist(outputDir, qualityPaths)
	if err != nil {
		return nil, fmt.Errorf("falha ao gerar master playlist: %w", err)
	}

	return &HLSOutput{
		MasterPlaylist: masterPath,
		Qualities:      qualityPaths,
	}, nil
}

func (c *HLSConverter) convertQuality(ctx context.Context, inputPath, outputDir string, q Quality) (string, error) {

	qualityDir := filepath.Join(outputDir, q.Name)
	if err := os.MkdirAll(qualityDir, 0o755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório %s: %w", q.Name, err)
	}

	playlistPath := filepath.Join(qualityDir, "index.m3u8")
	segmentPattern := filepath.Join(qualityDir, "segment_%03d.ts")
	scale := fmt.Sprintf("scale=%d:%d", q.Width, q.Height)

	err := c.runner.Run(ctx,
		"-i", inputPath,
		"-vf", scale,
		"-b:v", q.VideoBitrate,
		"-maxrate", q.MaxRate,
		"-bufsize", q.BufSize,
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPattern,
		playlistPath,
	)
	if err != nil {
		return "", err
	}

	return playlistPath, nil
}

func (c *HLSConverter) writeMasterPlaylist(outputDir string, qualityPaths map[string]string) (string, error) {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:3\n\n")

	// itera em Qualities para manter ordem crescente no master
	for _, q := range Qualities {
		playlistPath, ok := qualityPaths[q.Name]
		if !ok {
			continue
		}

		relPath, err := filepath.Rel(outputDir, playlistPath)
		if err != nil {
			return "", fmt.Errorf("falha ao calcular caminho relativo para %s: %w", q.Name, err)
		}

		sb.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s\n\n",
			q.Bandwidth, q.Width, q.Height, relPath,
		))
	}

	masterPath := filepath.Join(outputDir, "master.m3u8")
	if err := os.WriteFile(masterPath, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("falha ao escrever master.m3u8: %w", err)
	}

	return masterPath, nil
}
