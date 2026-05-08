package worker

import (
    "context"
    "log"
    "math"
    "sync"
    "time"

    "github.com/rayan-aguiar/video-processor/internal/queue"
)

type JobProcessor interface {
    ProcessJob(ctx context.Context, jobID string) error
}

type Config struct {
    QueueName             string
    WorkerCount           int
    DequeueTimeoutSeconds int
    JobTimeout            time.Duration
    BufferSize            int

    // Fase 5
    MaxRetries         int
    RetryBackoffBase   time.Duration
    RetryBackoffMax    time.Duration
    RetryQueue         string
    DeadLetterQueue    string
    RetrySweepInterval time.Duration
    RetryMoveBatch     int64
}

type Pool struct {
    adapter   queue.Adapter
    processor JobProcessor
    cfg       Config

    jobs   chan queue.JobMessage
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func NewPool(adapter queue.Adapter, processor JobProcessor, cfg Config) *Pool {
    if cfg.WorkerCount <= 0 {
        cfg.WorkerCount = 2
    }
    if cfg.DequeueTimeoutSeconds <= 0 {
        cfg.DequeueTimeoutSeconds = 2
    }
    if cfg.JobTimeout <= 0 {
        cfg.JobTimeout = 30 * time.Minute
    }
    if cfg.BufferSize <= 0 {
        cfg.BufferSize = cfg.WorkerCount * 2
    }

    // Fase 5 defaults
    if cfg.MaxRetries <= 0 {
        cfg.MaxRetries = 3
    }
    if cfg.RetryBackoffBase <= 0 {
        cfg.RetryBackoffBase = 5 * time.Second
    }
    if cfg.RetryBackoffMax <= 0 {
        cfg.RetryBackoffMax = 5 * time.Minute
    }
    if cfg.RetryQueue == "" {
        cfg.RetryQueue = cfg.QueueName + ":retry"
    }
    if cfg.DeadLetterQueue == "" {
        cfg.DeadLetterQueue = cfg.QueueName + ":dead"
    }
    if cfg.RetrySweepInterval <= 0 {
        cfg.RetrySweepInterval = 1 * time.Second
    }
    if cfg.RetryMoveBatch <= 0 {
        cfg.RetryMoveBatch = 100
    }

    return &Pool{
        adapter:   adapter,
        processor: processor,
        cfg:       cfg,
        jobs:      make(chan queue.JobMessage, cfg.BufferSize),
    }
}

func (p *Pool) Start(parent context.Context) {
    ctx, cancel := context.WithCancel(parent)
    p.cancel = cancel

    p.wg.Add(1)
    go p.dispatcher(ctx)

    // Promove retries vencidos do ZSET para a fila principal.
    p.wg.Add(1)
    go p.retryPromoter(ctx)

    for i := 0; i < p.cfg.WorkerCount; i++ {
        p.wg.Add(1)
        go p.worker(ctx, i+1)
    }

    log.Printf(
        "worker pool iniciado: workers=%d buffer=%d queue=%s retry_queue=%s dlq=%s max_retries=%d",
        p.cfg.WorkerCount,
        p.cfg.BufferSize,
        p.cfg.QueueName,
        p.cfg.RetryQueue,
        p.cfg.DeadLetterQueue,
        p.cfg.MaxRetries,
    )
}

func (p *Pool) Stop() {
    if p.cancel != nil {
        p.cancel()
    }
    p.wg.Wait()
    log.Println("worker pool finalizado")
}

func (p *Pool) dispatcher(ctx context.Context) {
    defer p.wg.Done()
    defer close(p.jobs)

    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        msg, err := p.adapter.DequeueBlocking(ctx, p.cfg.QueueName, p.cfg.DequeueTimeoutSeconds)
        if err != nil {
            log.Printf("dispatcher dequeue error: %v", err)
            continue
        }
        if msg == nil {
            continue
        }

        select {
        case <-ctx.Done():
            return
        case p.jobs <- *msg:
        }
    }
}

func (p *Pool) retryPromoter(ctx context.Context) {
    defer p.wg.Done()

    ticker := time.NewTicker(p.cfg.RetrySweepInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            moved, err := p.adapter.RequeueDue(ctx, p.cfg.RetryQueue, p.cfg.QueueName, p.cfg.RetryMoveBatch)
            if err != nil {
                log.Printf("retry promoter error: %v", err)
                continue
            }
            if moved > 0 {
                log.Printf("retry promoter moved=%d from=%s to=%s", moved, p.cfg.RetryQueue, p.cfg.QueueName)
            }
        }
    }
}

func (p *Pool) worker(ctx context.Context, workerID int) {
    defer p.wg.Done()

    for {
        select {
        case <-ctx.Done():
            return
        case msg, ok := <-p.jobs:
            if !ok {
                return
            }

            jobCtx, cancel := context.WithTimeout(ctx, p.cfg.JobTimeout)
            err := p.processor.ProcessJob(jobCtx, msg.JobID)
            cancel()

            if err != nil {
                p.handleFailure(ctx, workerID, msg, err)
                continue
            }

            log.Printf("worker=%d job=%s completed", workerID, msg.JobID)
        }
    }
}

func (p *Pool) handleFailure(ctx context.Context, workerID int, msg queue.JobMessage, processingErr error) {
    next := msg
    next.Attempts++
    now := time.Now().UTC()
    next.FailedAt = &now
    next.LastError = processingErr.Error()

    // excedeu retries: manda para dead-letter
    if next.Attempts > p.cfg.MaxRetries {
        if err := p.adapter.Enqueue(ctx, p.cfg.DeadLetterQueue, next); err != nil {
            log.Printf(
                "worker=%d job=%s dead-letter enqueue error=%v original=%v",
                workerID,
                msg.JobID,
                err,
                processingErr,
            )
            return
        }
        log.Printf(
            "worker=%d job=%s moved_to_dead_letter attempts=%d error=%v",
            workerID,
            msg.JobID,
            next.Attempts,
            processingErr,
        )
        return
    }

    delay := p.retryBackoff(next.Attempts)
    if err := p.adapter.EnqueueWithDelay(ctx, p.cfg.RetryQueue, next, delay); err != nil {
        log.Printf(
            "worker=%d job=%s retry enqueue error=%v original=%v",
            workerID,
            msg.JobID,
            err,
            processingErr,
        )
        return
    }

    log.Printf(
        "worker=%d job=%s scheduled_retry attempt=%d delay=%s error=%v",
        workerID,
        msg.JobID,
        next.Attempts,
        delay,
        processingErr,
    )
}

func (p *Pool) retryBackoff(attempt int) time.Duration {
    if attempt <= 0 {
        return p.cfg.RetryBackoffBase
    }

    // base * 2^(attempt-1)
    mult := math.Pow(2, float64(attempt-1))
    delay := time.Duration(float64(p.cfg.RetryBackoffBase) * mult)

    if delay > p.cfg.RetryBackoffMax {
        return p.cfg.RetryBackoffMax
    }
    return delay
}