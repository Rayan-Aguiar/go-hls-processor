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

type delayedCall struct {
    queueName string
    msg       queue.JobMessage
    delay     time.Duration
}

type fakeAdapter struct {
    mu      sync.Mutex
    results []dequeueResult
    idx     int

    enqueued    map[string][]queue.JobMessage
    delayed     []delayedCall
    requeueCall int
}

func newFakeAdapter(results []dequeueResult) *fakeAdapter {
    return &fakeAdapter{
        results:  results,
        enqueued: make(map[string][]queue.JobMessage),
    }
}

func (a *fakeAdapter) Enqueue(_ context.Context, queueName string, msg queue.JobMessage) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.enqueued[queueName] = append(a.enqueued[queueName], msg)
    return nil
}

func (a *fakeAdapter) EnqueueWithDelay(_ context.Context, queueName string, msg queue.JobMessage, delay time.Duration) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.delayed = append(a.delayed, delayedCall{
        queueName: queueName,
        msg:       msg,
        delay:     delay,
    })
    return nil
}

func (a *fakeAdapter) RequeueDue(_ context.Context, _ string, _ string, _ int64) (int64, error) {
    a.mu.Lock()
    a.requeueCall++
    a.mu.Unlock()
    return 0, nil
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
    p := NewPool(newFakeAdapter(nil), fakeProcessor{}, Config{QueueName: "video:jobs"})

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
    if p.cfg.MaxRetries != 3 {
        t.Fatalf("expected default MaxRetries=3, got %d", p.cfg.MaxRetries)
    }
    if p.cfg.RetryBackoffBase != 5*time.Second {
        t.Fatalf("expected default RetryBackoffBase=5s, got %s", p.cfg.RetryBackoffBase)
    }
    if p.cfg.RetryBackoffMax != 5*time.Minute {
        t.Fatalf("expected default RetryBackoffMax=5m, got %s", p.cfg.RetryBackoffMax)
    }
}

func TestPoolProcessesDequeuedJob(t *testing.T) {
    processed := make(chan string, 1)

    adapter := newFakeAdapter([]dequeueResult{
        {msg: &queue.JobMessage{JobID: "job-1"}},
    })

    processor := fakeProcessor{processFn: func(_ context.Context, jobID string) error {
        processed <- jobID
        return nil
    }}

    p := NewPool(adapter, processor, Config{
        QueueName:             "video:jobs",
        WorkerCount:           1,
        BufferSize:            1,
        JobTimeout:            time.Second,
        RetrySweepInterval:    50 * time.Millisecond,
        DequeueTimeoutSeconds: 1,
    })
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

func TestPoolSchedulesRetryOnFailure(t *testing.T) {
    adapter := newFakeAdapter([]dequeueResult{
        {msg: &queue.JobMessage{JobID: "job-retry", Attempts: 0}},
    })

    processor := fakeProcessor{processFn: func(_ context.Context, _ string) error {
        return errors.New("transient error")
    }}

    p := NewPool(adapter, processor, Config{
        QueueName:             "video:jobs",
        RetryQueue:            "video:jobs:retry",
        DeadLetterQueue:       "video:jobs:dead",
        WorkerCount:           1,
        BufferSize:            1,
        JobTimeout:            time.Second,
        MaxRetries:            3,
        RetryBackoffBase:      10 * time.Millisecond,
        RetryBackoffMax:       time.Second,
        RetrySweepInterval:    50 * time.Millisecond,
        DequeueTimeoutSeconds: 1,
    })
    p.Start(context.Background())
    defer p.Stop()

    time.Sleep(120 * time.Millisecond)

    adapter.mu.Lock()
    defer adapter.mu.Unlock()

    if len(adapter.delayed) != 1 {
        t.Fatalf("expected 1 delayed retry, got %d", len(adapter.delayed))
    }

    call := adapter.delayed[0]
    if call.queueName != "video:jobs:retry" {
        t.Fatalf("expected retry queue video:jobs:retry, got %s", call.queueName)
    }
    if call.msg.Attempts != 1 {
        t.Fatalf("expected attempts incremented to 1, got %d", call.msg.Attempts)
    }
    if call.msg.LastError == "" {
        t.Fatal("expected last error populated")
    }
    if call.msg.FailedAt == nil {
        t.Fatal("expected failedAt populated")
    }
}

func TestPoolSendsToDeadLetterWhenRetriesExceeded(t *testing.T) {
    adapter := newFakeAdapter([]dequeueResult{
        {msg: &queue.JobMessage{JobID: "job-dead", Attempts: 3}},
    })

    processor := fakeProcessor{processFn: func(_ context.Context, _ string) error {
        return errors.New("still failing")
    }}

    p := NewPool(adapter, processor, Config{
        QueueName:             "video:jobs",
        RetryQueue:            "video:jobs:retry",
        DeadLetterQueue:       "video:jobs:dead",
        WorkerCount:           1,
        BufferSize:            1,
        JobTimeout:            time.Second,
        MaxRetries:            3,
        RetryBackoffBase:      10 * time.Millisecond,
        RetryBackoffMax:       time.Second,
        RetrySweepInterval:    50 * time.Millisecond,
        DequeueTimeoutSeconds: 1,
    })
    p.Start(context.Background())
    defer p.Stop()

    time.Sleep(120 * time.Millisecond)

    adapter.mu.Lock()
    defer adapter.mu.Unlock()

    dlq := adapter.enqueued["video:jobs:dead"]
    if len(dlq) != 1 {
        t.Fatalf("expected 1 message in dead-letter, got %d", len(dlq))
    }
    if dlq[0].Attempts != 4 {
        t.Fatalf("expected attempts=4 in dead-letter, got %d", dlq[0].Attempts)
    }

    if len(adapter.delayed) != 0 {
        t.Fatalf("expected no delayed retry when dead-lettered, got %d", len(adapter.delayed))
    }
}

func TestPoolRetryPromoterRuns(t *testing.T) {
    adapter := newFakeAdapter(nil)

    processor := fakeProcessor{processFn: func(_ context.Context, _ string) error {
        return nil
    }}

    p := NewPool(adapter, processor, Config{
        QueueName:             "video:jobs",
        RetryQueue:            "video:jobs:retry",
        DeadLetterQueue:       "video:jobs:dead",
        WorkerCount:           1,
        BufferSize:            1,
        RetrySweepInterval:    20 * time.Millisecond,
        DequeueTimeoutSeconds: 1,
    })
    p.Start(context.Background())
    defer p.Stop()

    time.Sleep(90 * time.Millisecond)

    adapter.mu.Lock()
    calls := adapter.requeueCall
    adapter.mu.Unlock()

    if calls == 0 {
        t.Fatal("expected retry promoter to call RequeueDue at least once")
    }
}