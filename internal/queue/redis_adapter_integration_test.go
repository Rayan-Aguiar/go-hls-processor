//go:build integration

package queue

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestRedisAdapterIntegrationEnqueueDequeueLen(t *testing.T) {
	adapter, err := NewRedisAdapter(integrationRedisConfig())
	if err != nil {
		t.Skipf("skipping integration test, redis unavailable: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	ctx := context.Background()
	queueName := "video:jobs:test:" + strconv.FormatInt(time.Now().UnixNano(), 10)

	msg := JobMessage{
		JobID:      "job-int-1",
		Attempts:   0,
		EnqueuedAt: time.Now().UTC(),
	}

	if err := adapter.Enqueue(ctx, queueName, msg); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	n, err := adapter.Len(ctx, queueName)
	if err != nil {
		t.Fatalf("len failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected queue length 1, got %d", n)
	}

	dequeued, err := adapter.DequeueBlocking(ctx, queueName, 2)
	if err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}
	if dequeued == nil {
		t.Fatal("expected message, got nil")
	}

	if dequeued.JobID != msg.JobID {
		t.Fatalf("expected job id %s, got %s", msg.JobID, dequeued.JobID)
	}

	if dequeued.Attempts != msg.Attempts {
		t.Fatalf("expected attempts %d, got %d", msg.Attempts, dequeued.Attempts)
	}

	n, err = adapter.Len(ctx, queueName)
	if err != nil {
		t.Fatalf("len after dequeue failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected queue length 0, got %d", n)
	}
}

func integrationRedisConfig() RedisConfig {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		password = "videoproc2024"
	}

	db := 0
	if raw := os.Getenv("REDIS_DB"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			db = parsed
		}
	}

	return RedisConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
	}
}
