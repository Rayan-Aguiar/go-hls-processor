package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/rayan-aguiar/video-processor/internal/errors"
)

type mockAdapter struct {
	enqueueFn func(ctx context.Context, queueName string, msg JobMessage) error
}

func (m *mockAdapter) Enqueue(ctx context.Context, queueName string, msg JobMessage) error {
	if m.enqueueFn != nil {
		return m.enqueueFn(ctx, queueName, msg)
	}
	return nil
}

func (m *mockAdapter) DequeueBlocking(ctx context.Context, queueName string, timeoutSeconds int) (*JobMessage, error) {
	return nil, nil
}

func (m *mockAdapter) Len(ctx context.Context, queueName string) (int64, error) {
	return 0, nil
}

func (m *mockAdapter) Close() error {
	return nil
}

func TestProducerPublishJob(t *testing.T) {
	tests := []struct {
		name        string
		queueName   string
		jobID       string
		adapterErr  error
		wantErrKind error
	}{
		{
			name:        "returns invalid job id when empty",
			queueName:   "video:jobs",
			jobID:       "",
			wantErrKind: apperrors.ErrInvalidJobID,
		},
		{
			name:        "returns invalid queue name when empty",
			queueName:   "",
			jobID:       "job-1",
			wantErrKind: apperrors.ErrInvalidQueueName,
		},
		{
			name:        "wraps enqueue errors",
			queueName:   "video:jobs",
			jobID:       "job-2",
			adapterErr:  errors.New("redis down"),
			wantErrKind: apperrors.ErrQueueEnqueue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &mockAdapter{
				enqueueFn: func(_ context.Context, _ string, _ JobMessage) error {
					return tt.adapterErr
				},
			}

			p := NewProducer(adapter, tt.queueName)
			err := p.PublishJob(context.Background(), tt.jobID)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !errors.Is(err, tt.wantErrKind) {
				t.Fatalf("expected error kind %v, got %v", tt.wantErrKind, err)
			}
		})
	}
}

func TestProducerPublishJobSuccess(t *testing.T) {
	var gotQueue string
	var gotMessage JobMessage

	adapter := &mockAdapter{
		enqueueFn: func(_ context.Context, queueName string, msg JobMessage) error {
			gotQueue = queueName
			gotMessage = msg
			return nil
		},
	}

	p := NewProducer(adapter, "video:jobs")
	err := p.PublishJob(context.Background(), "job-123")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if gotQueue != "video:jobs" {
		t.Fatalf("expected queue video:jobs, got %s", gotQueue)
	}

	if gotMessage.JobID != "job-123" {
		t.Fatalf("expected job id job-123, got %s", gotMessage.JobID)
	}

	if gotMessage.Attempts != 0 {
		t.Fatalf("expected attempts 0, got %d", gotMessage.Attempts)
	}

	if gotMessage.EnqueuedAt.IsZero() {
		t.Fatal("expected EnqueuedAt to be set")
	}

	if gotMessage.EnqueuedAt.After(time.Now().UTC().Add(1 * time.Second)) {
		t.Fatalf("unexpected EnqueuedAt value: %s", gotMessage.EnqueuedAt)
	}
}

func (m *mockAdapter) EnqueueWithDelay(_ context.Context, _ string, _ JobMessage, _ time.Duration) error {
    return nil
}

func (m *mockAdapter) RequeueDue(_ context.Context, _ string, _ string, _ int64) (int64, error) {
    return 0, nil
}