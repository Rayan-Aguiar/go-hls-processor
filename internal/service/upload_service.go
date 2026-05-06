package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rayan-aguiar/video-processor/internal/validator"
)

type UploadService struct {
	tmpDir         string
	videoValidator *validator.VideoValidator
}

func New(tmpDir string) *UploadService {
	return &UploadService{
		tmpDir:         tmpDir,
		videoValidator: validator.New(),
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
}


func (s *UploadService) UploadAndValidateFile(input UploadFileInput) (*UploadFileOutput, error) {
	// Validar o arquivo
	if err := s.videoValidator.ValidateFile(input.Filename, input.FileSize); err != nil {
		return nil, err
	}

	// Gerar ID unico para o job
	jobID := uuid.New().String()

	// Criar diretorio para o Job
	jobDir := filepath.Join(s.tmpDir, jobID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return nil, fmt.Errorf("Falha ao criar diretorio do job: %w", err)
	}

	inputPath := filepath.Join(jobDir, input.Filename)
	file, err := os.Create(inputPath)
	if err != nil {
		return nil, fmt.Errorf("Falha ao criar arquivo de entrada: %w", err)
	}
	defer file.Close()

	// Copiar arquivo para disco
	if _, err := io.Copy(file, input.Reader); err != nil {
		os.Remove(inputPath)
		return nil, fmt.Errorf("Falha ao salvar arquivo: %w", err)
	}
	return &UploadFileOutput{
        JobID:     jobID,
        InputPath: inputPath,
    }, nil
}