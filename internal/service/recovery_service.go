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
    conn         *sql.DB
    adapter      queue.Adapter
    queueName    string
    stuckAfter   time.Duration
    pendingAfter time.Duration
    batchSize    int
}

func NewRecoveryService(conn *sql.DB, adapter queue.Adapter, queueName string, stuckAfter time.Duration, batchSize int) *RecoveryService {
    if stuckAfter <= 0 {
        stuckAfter = 2 * time.Minute
    }
    if batchSize <= 0 {
        batchSize = 100
    }
    pendingAfter := 15 * time.Second
    if stuckAfter > 0 && stuckAfter < pendingAfter {
        pendingAfter = stuckAfter
    }

    return &RecoveryService{
        conn:         conn,
        adapter:      adapter,
        queueName:    queueName,
        stuckAfter:   stuckAfter,
        pendingAfter: pendingAfter,
        batchSize:    batchSize,
    }
}

func (s *RecoveryService) Recover(ctx context.Context) (int, error) {
    processingCutoff := time.Now().Add(-s.stuckAfter)
    pendingCutoff := time.Now().Add(-s.pendingAfter)

    sqlConn, err := s.conn.Conn(ctx)
    if err != nil {
        return 0, fmt.Errorf("obter conexao dedicada para recovery: %w", err)
    }

    processingIDs, err := db.ListStuckProcessingJobs(sqlConn, processingCutoff, s.batchSize)
    if err != nil {
        _ = sqlConn.Close()
        return 0, fmt.Errorf("listar jobs presos processing: %w", err)
    }

    queueLen, err := s.adapter.Len(ctx, s.queueName)
    if err != nil {
        _ = sqlConn.Close()
        return 0, fmt.Errorf("obter tamanho da fila principal: %w", err)
    }

    pendingIDs := make([]string, 0)
    // Evita reenfileiramento repetido de jobs pending em cenários de backlog alto.
    // Só tenta recuperar pending quando a fila principal está vazia, que é quando
    // faz mais sentido suspeitar de orfandade.
    if queueLen == 0 {
        pendingIDs, err = db.ListStuckPendingJobs(sqlConn, pendingCutoff, s.batchSize)
        if err != nil {
            _ = sqlConn.Close()
            return 0, fmt.Errorf("listar jobs presos pending: %w", err)
        }
    }

    _ = sqlConn.Close()

    recovered := 0

    for _, jobID := range processingIDs {
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

    for _, jobID := range pendingIDs {
        msg := queue.JobMessage{
            JobID:      jobID,
            Attempts:   0,
            EnqueuedAt: time.Now().UTC(),
            LastError:  "recovered_from_stuck_pending",
        }

        if err := s.adapter.Enqueue(ctx, s.queueName, msg); err != nil {
            return recovered, fmt.Errorf("reenfileirar job pending %s: %w", jobID, err)
        }

        // Mantemos em pending e só atualizamos updated_at para evitar reenfileirar a cada sweep.
        if err := db.UpdateJobStatus(s.conn, jobID, "pending"); err != nil {
            return recovered, fmt.Errorf("atualizar status do job pending %s: %w", jobID, err)
        }

        recovered++
    }

    return recovered, nil
}