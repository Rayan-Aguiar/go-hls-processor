package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/ffmpeg"
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
	)

	cfg := worker.Config{
		QueueName:             envOrDefault("QUEUE_NAME", "video:jobs"),
		WorkerCount:           envIntOrDefault("WORKER_POOL_SIZE", 4),
		DequeueTimeoutSeconds: envIntOrDefault("QUEUE_DEQUEUE_TIMEOUT_SECONDS", 2),
		JobTimeout:            time.Duration(envIntOrDefault("WORKER_TIMEOUT_MINUTES", 30)) * time.Minute,
		BufferSize:            envIntOrDefault("WORKER_BUFFER_SIZE", 8),
	}

	pool := worker.NewPool(redisAdapter, processingAdapter{processor: processor}, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("sinal recebido, finalizando worker...")
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
