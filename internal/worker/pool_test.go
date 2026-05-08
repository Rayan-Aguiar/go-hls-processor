package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rayan-aguiar/video-processor/internal/queue"
)

type dequeueResult struct {
	msg *queue.JobMessage
	err error
}

type fakeAdapter struct {
	mu      sync.Mutex
	results []dequeueResult
	idx     int
}

func (a *fakeAdapter) Enqueue(_ context.Context, _ string, _ queue.JobMessage) error {
	return nil
}

func (a *fakeAdapter) DequeueBlocking(ctx context.Context, _ string, _ int) (*queue.JobMessage, error) {
	a.mu.Lock()
	if a.idx < len(a.results) {
		res := a.results[a.idx]
		a.idx++
		a.mu.Unlock()
		return res.msg, res.err
	}
	a.mu.Unlock()

	<-ctx.Done()
	return nil, ctx.Err()
}

func (a *fakeAdapter) Len(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (a *fakeAdapter) Close() error {
	return nil
}

type fakeProcessor struct {
	processFn func(ctx context.Context, jobID string) error
}

func (p fakeProcessor) ProcessJob(ctx context.Context, jobID string) error {
	if p.processFn != nil {
		return p.processFn(ctx, jobID)
	}
	return nil
}

func TestNewPoolAppliesDefaults(t *testing.T) {
	p := NewPool(&fakeAdapter{}, fakeProcessor{}, Config{})

	if p.cfg.WorkerCount != 2 {
		t.Fatalf("expected default WorkerCount=2, got %d", p.cfg.WorkerCount)
	}

	if p.cfg.DequeueTimeoutSeconds != 2 {
		t.Fatalf("expected default DequeueTimeoutSeconds=2, got %d", p.cfg.DequeueTimeoutSeconds)
	}

	if p.cfg.JobTimeout != 30*time.Minute {
		t.Fatalf("expected default JobTimeout=30m, got %s", p.cfg.JobTimeout)
	}

	if p.cfg.BufferSize != 4 {
		t.Fatalf("expected default BufferSize=4, got %d", p.cfg.BufferSize)
	}
}

func TestPoolProcessesDequeuedJob(t *testing.T) {
	processed := make(chan string, 1)

	adapter := &fakeAdapter{results: []dequeueResult{{msg: &queue.JobMessage{JobID: "job-1"}}}}
	processor := fakeProcessor{processFn: func(_ context.Context, jobID string) error {
		processed <- jobID
		return nil
	}}

	p := NewPool(adapter, processor, Config{QueueName: "video:jobs", WorkerCount: 1, BufferSize: 1, JobTimeout: time.Second})
	p.Start(context.Background())
	defer p.Stop()

	select {
	case jobID := <-processed:
		if jobID != "job-1" {
			t.Fatalf("expected processed job-1, got %s", jobID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job processing")
	}
}

func TestPoolContinuesAfterDequeueError(t *testing.T) {
	processed := make(chan string, 1)

	adapter := &fakeAdapter{results: []dequeueResult{
		{err: errors.New("temporary dequeue error")},
		{msg: &queue.JobMessage{JobID: "job-2"}},
	}}

	processor := fakeProcessor{processFn: func(_ context.Context, jobID string) error {
		processed <- jobID
		return nil
	}}

	p := NewPool(adapter, processor, Config{QueueName: "video:jobs", WorkerCount: 1, BufferSize: 1, JobTimeout: time.Second})
	p.Start(context.Background())
	defer p.Stop()

	select {
	case jobID := <-processed:
		if jobID != "job-2" {
			t.Fatalf("expected processed job-2, got %s", jobID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job processing")
	}
}

func TestPoolAppliesJobTimeout(t *testing.T) {
	deadlineObserved := make(chan bool, 1)
	finished := make(chan struct{}, 1)

	adapter := &fakeAdapter{results: []dequeueResult{{msg: &queue.JobMessage{JobID: "job-timeout"}}}}
	processor := fakeProcessor{processFn: func(ctx context.Context, _ string) error {
		deadline, ok := ctx.Deadline()
		deadlineObserved <- ok && time.Until(deadline) > 0
		<-ctx.Done()
		finished <- struct{}{}
		return ctx.Err()
	}}

	p := NewPool(adapter, processor, Config{
		QueueName:   "video:jobs",
		WorkerCount: 1,
		BufferSize:  1,
		JobTimeout:  30 * time.Millisecond,
	})

	p.Start(context.Background())
	defer p.Stop()

	select {
	case ok := <-deadlineObserved:
		if !ok {
			t.Fatal("expected worker context with deadline")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for deadline observation")
	}

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for timeout-driven finish")
	}
}
