package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	apperrors "github.com/rayan-aguiar/video-processor/internal/errors"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type RedisAdapter struct {
	client *redis.Client
}

func NewRedisAdapter(cfg RedisConfig) (*RedisAdapter, error) {
	if cfg.Host == "" || cfg.Port == "" {
		return nil, apperrors.New(apperrors.ErrRedisConnect, "queue.redis.new", fmt.Errorf("host and port are required"))
	}

	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return nil, apperrors.New(apperrors.ErrRedisConnect, "queue.redis.new", fmt.Errorf("invalid redis port: %w", err))
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     20,
		MinIdleConns: 5,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, apperrors.New(apperrors.ErrRedisConnect, "queue.redis.new", err)
	}

	return &RedisAdapter{client: client}, nil
}

func (r *RedisAdapter) Enqueue(ctx context.Context, queueName string, msg JobMessage) error {
	if queueName == "" {
		return apperrors.New(apperrors.ErrInvalidQueueName, "queue.redis.enqueue", nil)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return apperrors.New(apperrors.ErrQueueEncode, "queue.redis.enqueue", err)
	}

	if err := r.client.RPush(ctx, queueName, payload).Err(); err != nil {
		return apperrors.New(apperrors.ErrQueueEnqueue, "queue.redis.enqueue", err)
	}

	return nil
}

func (r *RedisAdapter) DequeueBlocking(ctx context.Context, queueName string, timeoutSeconds int) (*JobMessage, error) {
	if queueName == "" {
		return nil, apperrors.New(apperrors.ErrInvalidQueueName, "queue.redis.dequeue", nil)
	}

	if timeoutSeconds <= 0 {
		return nil, apperrors.New(apperrors.ErrInvalidTimeout, "queue.redis.dequeue", nil)
	}

	result, err := r.client.BLPop(ctx, time.Duration(timeoutSeconds)*time.Second, queueName).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, apperrors.New(apperrors.ErrQueueDequeue, "queue.redis.dequeue", err)
	}

	// BLPOP retorna [queueName, payload]
	if len(result) < 2 {
		return nil, apperrors.New(apperrors.ErrQueueDecode, "queue.redis.dequeue", fmt.Errorf("invalid BLPOP response"))
	}

	var msg JobMessage
	if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
		return nil, apperrors.New(apperrors.ErrQueueDecode, "queue.redis.dequeue", err)
	}

	return &msg, nil
}

func (r *RedisAdapter) Len(ctx context.Context, queueName string) (int64, error) {
	if queueName == "" {
		return 0, apperrors.New(apperrors.ErrInvalidQueueName, "queue.redis.len", nil)
	}

	n, err := r.client.LLen(ctx, queueName).Result()
	if err != nil {
		return 0, apperrors.New(apperrors.ErrQueueLen, "queue.redis.len", err)
	}
	return n, nil
}

func (r *RedisAdapter) Close() error {
	if err := r.client.Close(); err != nil {
		return apperrors.New(apperrors.ErrQueueClose, "queue.redis.close", err)
	}
	return nil
}
