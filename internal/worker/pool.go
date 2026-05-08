package worker

import (
	"context"
	"log"
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

	for i := 0; i < p.cfg.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i+1)
	}

	log.Printf("worker pool iniciado: workers=%d buffer=%d queue=%s",
		p.cfg.WorkerCount, p.cfg.BufferSize, p.cfg.QueueName)
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
				log.Printf("worker=%d job=%s error=%v", workerID, msg.JobID, err)
				continue
			}

			log.Printf("worker=%d job=%s completed", workerID, msg.JobID)
		}
	}
}
