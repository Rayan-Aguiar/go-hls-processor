package queue

import (
	"context"
	"time"
)

type Adapter interface {
	Enqueue(ctx context.Context, queueName string, msg JobMessage) error
	DequeueBlocking(ctx context.Context, queueName string, timeoutSeconds int) (*JobMessage, error)
	Len(ctx context.Context, queueName string) (int64, error)

	EnqueueWithDelay(ctx context.Context, retryQueue string, msg JobMessage, delay time.Duration) error
	RequeueDue(ctx context.Context, retryQueue, targetQueue string, maxItems int64) (int64, error)

	Close() error
}
