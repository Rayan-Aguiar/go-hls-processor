package validator

import "testing"

func TestVideoValidatorValidateFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		fileSize int64
		wantErr  bool
	}{
		{
			name:     "accepts supported extension in uppercase",
			filename: "video.MP4",
			fileSize: 1024,
			wantErr:  false,
		},
		{
			name:     "rejects unsupported extension",
			filename: "video.txt",
			fileSize: 1024,
			wantErr:  true,
		},
		{
			name:     "rejects empty file",
			filename: "video.mp4",
			fileSize: 0,
			wantErr:  true,
		},
		{
			name:     "rejects file larger than 5GB",
			filename: "video.mp4",
			fileSize: int64(5*1024*1024*1024) + 1,
			wantErr:  true,
		},
	}

	v := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateFile(tt.filename, tt.fileSize)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
