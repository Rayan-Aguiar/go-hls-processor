package ffmpeg


import (
    "context"
    "fmt"
    "os"
    "path/filepath"
)

type ThumbnailGenerator struct {
    runner Runner
}

func NewThumbnailGenerator(runner Runner) *ThumbnailGenerator {
    return &ThumbnailGenerator{runner: runner}
}

// Generate extrai o primeiro frame do vídeo como thumbnail JPEG.
func (g *ThumbnailGenerator) Generate(ctx context.Context, inputPath, outputDir string) (string, error) {
    if err := os.MkdirAll(outputDir, 0o755); err != nil {
        return "", fmt.Errorf("falha ao criar diretório de thumbnail: %w", err)
    }

    thumbnailPath := filepath.Join(outputDir, "thumbnail.jpg")

    err := g.runner.Run(ctx,
        "-i", inputPath,
        "-ss", "00:00:00",   // primeiro frame
        "-vframes", "1",     // apenas 1 frame
        "-q:v", "2",         // qualidade JPEG (2 = alta, 31 = baixa)
        thumbnailPath,
    )
    if err != nil {
        return "", fmt.Errorf("falha ao gerar thumbnail: %w", err)
    }

    return thumbnailPath, nil
}