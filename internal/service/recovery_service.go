package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/queue"
)

type RecoveryService struct {
    conn       *sql.DB
    adapter    queue.Adapter
    queueName  string
    stuckAfter time.Duration
    batchSize  int
}

func NewRecoveryService(conn *sql.DB, adapter queue.Adapter, queueName string, stuckAfter time.Duration, batchSize int) *RecoveryService {
    if stuckAfter <= 0 {
        stuckAfter = 2 * time.Minute
    }
    if batchSize <= 0 {
        batchSize = 100
    }

    return &RecoveryService{
        conn:       conn,
        adapter:    adapter,
        queueName:  queueName,
        stuckAfter: stuckAfter,
        batchSize:  batchSize,
    }
}

func (s *RecoveryService) Recover(ctx context.Context) (int, error) {
    cutoff := time.Now().Add(-s.stuckAfter)

    sqlConn, err := s.conn.Conn(ctx)
    if err != nil {
        return 0, fmt.Errorf("obter conexao dedicada para recovery: %w", err)
    }

    ids, err := db.ListStuckProcessingJobs(sqlConn, cutoff, s.batchSize)
    _ = sqlConn.Close()
    if err != nil {
        return 0, fmt.Errorf("listar jobs presos: %w", err)
    }

    recovered := 0

    for _, jobID := range ids {
        msg := queue.JobMessage{
            JobID:      jobID,
            Attempts:   0,
            EnqueuedAt: time.Now().UTC(),
            LastError:  "recovered_from_stuck_processing",
        }

        if err := s.adapter.Enqueue(ctx, s.queueName, msg); err != nil {
            return recovered, fmt.Errorf("reenfileirar job %s: %w", jobID, err)
        }

        if err := db.UpdateJobStatus(s.conn, jobID, "pending"); err != nil {
            return recovered, fmt.Errorf("atualizar status do job %s para pending: %w", jobID, err)
        }

        recovered++
    }

    return recovered, nil
}