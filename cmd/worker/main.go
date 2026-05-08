package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/ffmpeg"
	"github.com/rayan-aguiar/video-processor/internal/observability"
	"github.com/rayan-aguiar/video-processor/internal/progress"
	"github.com/rayan-aguiar/video-processor/internal/queue"
	"github.com/rayan-aguiar/video-processor/internal/service"
	"github.com/rayan-aguiar/video-processor/internal/worker"
)

type processingAdapter struct {
	processor *service.ProcessingService
}

func (a processingAdapter) ProcessJob(ctx context.Context, jobID string) error {
	_, err := a.processor.ProcessJob(ctx, jobID)
	return err
}

func main() {
	_ = godotenv.Load()

	databaseURL := envOrDefault("DATABASE_URL", "postgres://videoproc:videoproc@localhost:5432/video_processor?sslmode=disable")
	conn, err := db.Open(databaseURL)
	if err != nil {
		log.Fatalf("conectar no postgresql: %v", err)
	}
	defer conn.Close()

	redisAdapter, err := queue.NewRedisAdapter(queue.RedisConfig{
		Host:     envOrDefault("REDIS_HOST", "localhost"),
		Port:     envOrDefault("REDIS_PORT", "6379"),
		Password: envOrDefault("REDIS_PASSWORD", "videoproc2024"),
		DB:       envIntOrDefault("REDIS_DB", 0),
	})
	if err != nil {
		log.Fatalf("conectar no redis: %v", err)
	}
	defer redisAdapter.Close()

	ffRunner := ffmpeg.NewRunner("")
	hlsConverter := ffmpeg.NewHLSConverter(ffRunner)
	thumbGenerator := ffmpeg.NewThumbnailGenerator(ffRunner)

	processor := service.NewProcessingService(
		conn,
		envOrDefault("DATA_DIR", "./data"),
		hlsConverter,
		thumbGenerator,
	).WithProgress(progress.NewPublisher(redisAdapter.Client()))

	defaultWorkers := defaultWorkerPoolSize()
	workerCount := envIntOrDefault("WORKER_POOL_SIZE", defaultWorkers)
	if workerCount <= 0 {
		workerCount = defaultWorkers
	}

	defaultBuffer := workerCount * 3
	bufferSize := envIntOrDefault("WORKER_BUFFER_SIZE", defaultBuffer)
	if bufferSize <= 0 {
		bufferSize = defaultBuffer
	}

	jobTimeoutMinutes := envIntOrDefault("WORKER_TIMEOUT_MINUTES", 30)
	if jobTimeoutMinutes <= 0 {
		jobTimeoutMinutes = 30
	}

	cfg := worker.Config{
		QueueName:             envOrDefault("QUEUE_NAME", "video:jobs"),
		WorkerCount:           workerCount,
		DequeueTimeoutSeconds: envIntOrDefault("QUEUE_DEQUEUE_TIMEOUT_SECONDS", 2),
		JobTimeout:            time.Duration(jobTimeoutMinutes) * time.Minute,
		BufferSize:            bufferSize,

		MaxRetries:         envIntOrDefault("MAX_RETRIES", 3),
		RetryBackoffBase:   time.Duration(envIntOrDefault("RETRY_BACKOFF_SECONDS", 5)) * time.Second,
		RetryBackoffMax:    time.Duration(envIntOrDefault("RETRY_BACKOFF_MAX_SECONDS", 300)) * time.Second,
		RetryQueue:         envOrDefault("RETRY_QUEUE", "video:jobs:retry"),
		DeadLetterQueue:    envOrDefault("DEAD_LETTER_QUEUE", "video:jobs:dead"),
		RetrySweepInterval: time.Duration(envIntOrDefault("RETRY_SWEEP_INTERVAL_SECONDS", 1)) * time.Second,
		RetryMoveBatch:     int64(envIntOrDefault("RETRY_MOVE_BATCH", 100)),

		RecoverySweepInterval: time.Duration(envIntOrDefault("RECOVERY_SWEEP_INTERVAL_SECONDS", 5)) * time.Second,
	}

	recovery := service.NewRecoveryService(
		conn,
		redisAdapter,
		cfg.QueueName,
		time.Duration(envIntOrDefault("RECOVERY_STUCK_AFTER_SECONDS", 120))*time.Second,
		envIntOrDefault("RECOVERY_BATCH_SIZE", 100),
	)

	pool := worker.NewPool(redisAdapter, processingAdapter{processor: processor}, cfg).WithRecovery(recovery)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	metricsPort := envOrDefault("WORKER_METRICS_PORT", "9101")
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:              ":" + metricsPort,
		Handler:           observability.MetricsMiddleware(metricsMux),
		ReadHeaderTimeout: 3 * time.Second,
	}
	go func() {
		log.Printf("worker metrics ouvindo em http://localhost:%s/metrics", metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("erro no servidor de metrics do worker: %v", err)
		}
	}()

	pool.Start(ctx)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("sinal recebido, finalizando worker...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro ao encerrar servidor de metrics do worker: %v", err)
	}
	pool.Stop()
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return v
}

func defaultWorkerPoolSize() int {
	cpuHalf := runtime.NumCPU() / 2
	if cpuHalf < 2 {
		return 2
	}
	if cpuHalf > 6 {
		return 6
	}
	return cpuHalf
}
