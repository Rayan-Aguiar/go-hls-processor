package queue

import "time"

type JobMessage struct {
    JobID      string     `json:"job_id"`
    Attempts   int        `json:"attempts"`
    EnqueuedAt time.Time  `json:"enqueued_at"`
    LastError  string     `json:"last_error,omitempty"`
    FailedAt   *time.Time `json:"failed_at,omitempty"`
}