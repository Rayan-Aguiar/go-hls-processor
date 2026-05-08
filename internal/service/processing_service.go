package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/ffmpeg"
	"github.com/rayan-aguiar/video-processor/internal/models"
)

type HLSConverter interface {
	Convert(ctx context.Context, inputPath, outputDir string, onQuality func(name string, done, total int)) (*ffmpeg.HLSOutput, error)
}

// ProgressReporter recebe atualizações de progresso do processamento.
// Implementações podem publicar via Redis pub/sub, log, etc.
type ProgressReporter interface {
	Report(ctx context.Context, jobID string, percent int, stage string)
	Done(ctx context.Context, jobID string)
	Fail(ctx context.Context, jobID string, reason string)
}

type ThumbnailGenerator interface {
	Generate(ctx context.Context, inputPath, outputPath string) (string, error)
}

type ProcessingService struct {
	conn      *sql.DB
	baseDir   string
	hls       HLSConverter
	thumbnail ThumbnailGenerator
	progress  ProgressReporter
}

func NewProcessingService(conn *sql.DB, baseDir string, hls HLSConverter, thumbnail ThumbnailGenerator) *ProcessingService {
	return &ProcessingService{
		conn:      conn,
		baseDir:   baseDir,
		hls:       hls,
		thumbnail: thumbnail,
	}
}

// WithProgress injeta um ProgressReporter opcional. Nil é aceito (sem relatório).
func (s *ProcessingService) WithProgress(r ProgressReporter) *ProcessingService {
	s.progress = r
	return s
}

func (s *ProcessingService) report(ctx context.Context, jobID string, pct int, stage string) {
	if s.progress != nil {
		s.progress.Report(ctx, jobID, pct, stage)
	}
}

type ProcessJobOutput struct {
	JobID          string
	OutputDir      string
	MasterPlaylist string
	ThumbnailPath  string
}

func (s *ProcessingService) ProcessJob(ctx context.Context, jobID string) (*ProcessJobOutput, error) {
	log.Printf("processing: job=%s buscando job no banco", jobID)
	job, err := db.GetJobByID(ctx, s.conn, jobID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar job: %w", err)
	}

	if err := db.UpdateJobStatus(s.conn, jobID, string(models.JobStatusProcessing)); err != nil {
		return nil, fmt.Errorf("falha ao atualizar status para processing: %w", err)
	}
	log.Printf("processing: job=%s status=processing", jobID)

	s.report(ctx, jobID, 5, "iniciando processamento")

	outputDir := filepath.Join(s.baseDir, jobID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		_ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
		if s.progress != nil {
			s.progress.Fail(ctx, jobID, "falha ao criar diretório")
		}
		return nil, fmt.Errorf("falha ao criar diretório de saída: %w", err)
	}

	hlsOutput, err := s.hls.Convert(ctx, job.InputPath, outputDir, func(name string, done, total int) {
		pct := done * 80 / total // 360p=20%, 480p=40%, 720p=60%, 1080p=80%
		log.Printf("processing: job=%s qualidade=%s progresso=%d/%d pct=%d", jobID, name, done, total, pct)
		s.report(ctx, jobID, pct, name+" concluído")
	})
	if err != nil {
		_ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
		if s.progress != nil {
			s.progress.Fail(ctx, jobID, "falha na conversão HLS")
		}
		return nil, fmt.Errorf("falha ao converter para HLS: %w", err)
	}
	log.Printf("processing: job=%s hls concluido", jobID)
	s.report(ctx, jobID, 90, "gerando thumbnail")
	thumbnailPath, err := s.thumbnail.Generate(ctx, job.InputPath, outputDir)
	if err != nil {
		_ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
		if s.progress != nil {
			s.progress.Fail(ctx, jobID, "falha ao gerar thumbnail")
		}
		return nil, fmt.Errorf("falha ao gerar thumbnail: %w", err)
	}
	if err := db.UpdateJobOutputDir(s.conn, jobID, outputDir); err != nil {
		_ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
		return nil, fmt.Errorf("falha ao atualizar output_dir: %w", err)
	}

	if err := db.UpdateJobStatus(s.conn, jobID, models.JobStatusCompleted.String()); err != nil {
		return nil, fmt.Errorf("falha ao atualizar status para completed: %w", err)
	}
	log.Printf("processing: job=%s status=completed", jobID)

	if s.progress != nil {
		s.progress.Done(ctx, jobID)
	}
	return &ProcessJobOutput{
		JobID:          jobID,
		OutputDir:      outputDir,
		MasterPlaylist: hlsOutput.MasterPlaylist,
		ThumbnailPath:  thumbnailPath,
	}, nil
}
