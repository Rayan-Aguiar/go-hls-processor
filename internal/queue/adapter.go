package queue

import "context"

type Adapter interface {
	Enqueue(ctx context.Context, queueName string, msg JobMessage) error
	DequeueBlocking(ctx context.Context, queueName string, timeoutSeconds int) (*JobMessage, error)
	Len(ctx context.Context, queueName string) (int64, error)
	Close() error
}
