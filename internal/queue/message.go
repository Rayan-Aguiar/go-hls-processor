package queue

import "time"

type JobMessage struct {
	JobID      string    `json:"job_id"`
	Attempts   int       `json:"attempts"`
	EnqueuedAt time.Time `json:"enqueued_at"`
}
