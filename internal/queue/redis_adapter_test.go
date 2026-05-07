package queue

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/rayan-aguiar/video-processor/internal/errors"
)

func TestNewRedisAdapterValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  RedisConfig
	}{
		{
			name: "missing host",
			cfg: RedisConfig{
				Host: "",
				Port: "6379",
			},
		},
		{
			name: "missing port",
			cfg: RedisConfig{
				Host: "localhost",
				Port: "",
			},
		},
		{
			name: "invalid port",
			cfg: RedisConfig{
				Host: "localhost",
				Port: "abc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRedisAdapter(tt.cfg)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !errors.Is(err, apperrors.ErrRedisConnect) {
				t.Fatalf("expected ErrRedisConnect, got %v", err)
			}
		})
	}
}

func TestRedisAdapterInputValidation(t *testing.T) {
	r := &RedisAdapter{}
	ctx := context.Background()

	if err := r.Enqueue(ctx, "", JobMessage{JobID: "job-1"}); !errors.Is(err, apperrors.ErrInvalidQueueName) {
		t.Fatalf("expected ErrInvalidQueueName on enqueue, got %v", err)
	}

	if _, err := r.DequeueBlocking(ctx, "", 1); !errors.Is(err, apperrors.ErrInvalidQueueName) {
		t.Fatalf("expected ErrInvalidQueueName on dequeue, got %v", err)
	}

	if _, err := r.DequeueBlocking(ctx, "video:jobs", 0); !errors.Is(err, apperrors.ErrInvalidTimeout) {
		t.Fatalf("expected ErrInvalidTimeout on dequeue, got %v", err)
	}

	if _, err := r.Len(ctx, ""); !errors.Is(err, apperrors.ErrInvalidQueueName) {
		t.Fatalf("expected ErrInvalidQueueName on len, got %v", err)
	}
}
