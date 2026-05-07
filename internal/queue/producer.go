package queue

import (
	"context"
	"time"

	apperrors "github.com/rayan-aguiar/video-processor/internal/errors"
)

type Producer struct {
	adapter   Adapter
	queueName string
}

func NewProducer(adapter Adapter, queueName string) *Producer {
	return &Producer{
		adapter:   adapter,
		queueName: queueName,
	}
}

func (p *Producer) PublishJob(ctx context.Context, jobID string) error {
	if jobID == "" {
		return apperrors.New(apperrors.ErrInvalidJobID, "queue.producer.publish", nil)
	}

	if p.queueName == "" {
		return apperrors.New(apperrors.ErrInvalidQueueName, "queue.producer.publish", nil)
	}

	msg := JobMessage{
		JobID:      jobID,
		Attempts:   0,
		EnqueuedAt: time.Now().UTC(),
	}

	if err := p.adapter.Enqueue(ctx, p.queueName, msg); err != nil {
		return apperrors.New(apperrors.ErrQueueEnqueue, "queue.producer.publish", err)
	}
	return nil
}
