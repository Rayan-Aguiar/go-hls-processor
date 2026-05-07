package service

import (
    "context"
    "database/sql"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "time"

    "github.com/google/uuid"
    "github.com/rayan-aguiar/video-processor/internal/db"
    "github.com/rayan-aguiar/video-processor/internal/models"
    "github.com/rayan-aguiar/video-processor/internal/validator"
)

type JobPublisher interface {
    PublishJob(ctx context.Context, jobID string) error
}

type UploadService struct {
    tmpDir         string
    videoValidator *validator.VideoValidator
    conn           *sql.DB
    publisher      JobPublisher
}

func New(tmpDir string, conn *sql.DB, publisher JobPublisher) *UploadService {
    return &UploadService{
        tmpDir:         tmpDir,
        videoValidator: validator.New(),
        conn:           conn,
        publisher:      publisher,
    }
}

type UploadFileInput struct {
    Filename string
    FileSize int64
    Reader   io.Reader
}

type UploadFileOutput struct {
    JobID     string
    InputPath string
    Status    string
}

func (s *UploadService) UploadAndValidateFile(ctx context.Context, input UploadFileInput) (*UploadFileOutput, error) {
    if err := s.validateInput(input); err != nil {
        return nil, err
    }

    jobID := s.newJobID()

    jobDir, err := s.createJobDir(jobID)
    if err != nil {
        return nil, err
    }

    inputPath, err := s.saveInputFile(jobDir, input)
    if err != nil {
        return nil, err
    }

    job := s.buildPendingJob(jobID, inputPath)

    if err := s.persistJob(job); err != nil {
        _ = os.RemoveAll(jobDir)
        return nil, err
    }

    if err := s.enqueueJob(ctx, job.ID); err != nil {
        _ = s.rollbackJob(job.ID, jobDir)
        return nil, err
    }

    return s.buildUploadOutput(job), nil
}

func (s *UploadService) validateInput(input UploadFileInput) error {
    return s.videoValidator.ValidateFile(input.Filename, input.FileSize)
}

func (s *UploadService) newJobID() string {
    return uuid.New().String()
}

func (s *UploadService) createJobDir(jobID string) (string, error) {
    jobDir := filepath.Join(s.tmpDir, jobID)
    if err := os.MkdirAll(jobDir, 0o755); err != nil {
        return "", fmt.Errorf("falha ao criar diretório do job: %w", err)
    }

    return jobDir, nil
}

func (s *UploadService) saveInputFile(jobDir string, input UploadFileInput) (string, error) {
    inputPath := filepath.Join(jobDir, input.Filename)

    file, err := os.Create(inputPath)
    if err != nil {
        return "", fmt.Errorf("falha ao criar arquivo: %w", err)
    }
    defer file.Close()

    if _, err = io.Copy(file, input.Reader); err != nil {
        _ = os.Remove(inputPath)
        return "", fmt.Errorf("falha ao salvar arquivo: %w", err)
    }

    return inputPath, nil
}

func (s *UploadService) buildPendingJob(jobID string, inputPath string) models.Job {
    return models.Job{
        ID:        jobID,
        Status:    models.JobStatusPending,
        InputPath: inputPath,
        CreatedAt: time.Now(),
    }
}

func (s *UploadService) persistJob(job models.Job) error {
    dbJob := db.Job{
        ID:        job.ID,
        Status:    string(job.Status),
        InputPath: job.InputPath,
        CreatedAt: job.CreatedAt,
    }

    if err := db.InsertJob(s.conn, dbJob); err != nil {
        return fmt.Errorf("falha ao salvar job no banco: %w", err)
    }

    return nil
}

func (s *UploadService) enqueueJob(ctx context.Context, jobID string) error {
    if s.publisher == nil {
        return fmt.Errorf("publisher não configurado")
    }

    if err := s.publisher.PublishJob(ctx, jobID); err != nil {
        return fmt.Errorf("falha ao publicar job na fila: %w", err)
    }

    return nil
}

func (s *UploadService) rollbackJob(jobID string, jobDir string) error {
    var rollbackErr error

    if err := db.DeleteJob(s.conn, jobID); err != nil {
        rollbackErr = fmt.Errorf("falha ao remover job do banco no rollback: %w", err)
    }

    if err := os.RemoveAll(jobDir); err != nil && rollbackErr == nil {
        rollbackErr = fmt.Errorf("falha ao remover arquivos do job no rollback: %w", err)
    }

    return rollbackErr
}

func (s *UploadService) buildUploadOutput(job models.Job) *UploadFileOutput {
    return &UploadFileOutput{
        JobID:     job.ID,
        InputPath: job.InputPath,
        Status:    string(job.Status),
    }
}