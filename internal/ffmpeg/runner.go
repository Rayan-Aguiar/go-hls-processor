package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
)

// Runner é a abstração para execução de comandos ffmpeg.
// Depender desta interface, e não do binário diretamente,
// permite trocar a implementação em testes sem subir um processo real.
type Runner interface {
    Run(ctx context.Context, args ...string) error
}

type FFmpegRunner struct {
	binPath string
}


// NewRunner cria um runner que chama o binário ffmpeg.
// Se binPath for vazio, usa "ffmpeg" direto do PATH.
func NewRunner(binPath string) *FFmpegRunner{
	if binPath == "" {
		binPath = "ffmpeg"
	}
	return &FFmpegRunner{binPath: binPath}
}

func (r *FFmpegRunner) Run(ctx context.Context, args ...string) error {
    cmd := exec.CommandContext(ctx, r.binPath, args...)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("ffmpeg: %w\noutput: %s", err, string(out))
    }
    return nil
}
