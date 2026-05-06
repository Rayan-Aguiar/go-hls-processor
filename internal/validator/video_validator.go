package validator

import (
	"fmt"
	"path/filepath"
	"strings"
)

var SupportedVideoFormats = []string{
	".mp4",
	".avi",
	".mkv",
	".mov",
	".flv",
	".wmv",
	".webm",
	".mpeg",
}

type VideoValidator struct{}

func New() *VideoValidator {
	return &VideoValidator{}
}

func (v *VideoValidator) ValidateFile(filename string, fileSize int64) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if !isSupportedFormat(ext) {
		return fmt.Errorf("Formato de video não suportado: %s", ext)
	}

	maxSize := int64(5 * 1024 * 1024 * 1024) // 5GB
	if fileSize > maxSize {
		return fmt.Errorf("O arquivo excede o tamanho máximo permitido de 5GB")
	}

	if fileSize == 0 {
		return fmt.Errorf("O arquivo está vazio")
	}

	return nil
}

func isSupportedFormat(ext string) bool {
	for _, fmt := range SupportedVideoFormats {
		if ext == fmt {
			return true
		}
	}
	return false
}
