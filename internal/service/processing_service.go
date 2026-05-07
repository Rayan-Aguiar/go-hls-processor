package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/ffmpeg"
	"github.com/rayan-aguiar/video-processor/internal/models"
)

type HLSConverter interface {
	Convert(ctx context.Context, inputPath, outputDir string) (*ffmpeg.HLSOutput, error)
}

type ThumbnailGenerator interface {
	Generate(ctx context.Context, inputPath, outputPath string) (string, error)
}

type ProcessingService struct {
	conn    *sql.DB
	baseDir string
	hls     HLSConverter
	thumbnail   ThumbnailGenerator
}

func NewProcessingService(conn *sql.DB, baseDir string, hls HLSConverter, thumbnail ThumbnailGenerator) *ProcessingService {
	return &ProcessingService{
		conn:    conn,
		baseDir: baseDir,
		hls:     hls,
		thumbnail:   thumbnail,
	}
}

type ProcessJobOutput struct {
	JobID          string
	OutputDir      string
	MasterPlaylist string
	ThumbnailPath  string
}

func (s *ProcessingService) ProcessJob(ctx context.Context, jobID string) (*ProcessJobOutput, error) {
	job, err := db.GetJobByID(ctx, s.conn, jobID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar job: %w", err)
	}

	if err := db.UpdateJobStatus(s.conn, jobID, string(models.JobStatusProcessing)); err != nil {
		return nil, fmt.Errorf("falha ao atualizar status para processing: %w", err)
	}

	outputDir := filepath.Join(s.baseDir, jobID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
        _ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
        return nil, fmt.Errorf("falha ao criar diretório de saída: %w", err)
    }

	hlsOutput, err := s.hls.Convert(ctx, job.InputPath, outputDir)
	if err != nil {
		_ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
		return nil, fmt.Errorf("falha ao converter para HLS: %w", err)
	}
	thumbnailPath, err := s.thumbnail.Generate(ctx, job.InputPath, outputDir)
    if err != nil {
        _ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
        return nil, fmt.Errorf("falha ao gerar thumbnail: %w", err)
    }
	if err := db.UpdateJobOutputDir(s.conn, jobID, outputDir); err != nil {
        _ = db.UpdateJobStatus(s.conn, jobID, models.JobStatusFailed.String())
        return nil, fmt.Errorf("falha ao atualizar output_dir: %w", err)
    }

	if err := db.UpdateJobStatus(s.conn, jobID, models.JobStatusCompleted.String()); err != nil {
        return nil, fmt.Errorf("falha ao atualizar status para completed: %w", err)
    }
	return &ProcessJobOutput{
		JobID:          jobID,
		OutputDir:      outputDir,
		MasterPlaylist: hlsOutput.MasterPlaylist,
		ThumbnailPath:  thumbnailPath,
	}, nil
}