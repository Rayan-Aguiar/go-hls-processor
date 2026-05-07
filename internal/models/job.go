package models

import "time"

type Job struct {
	ID         string
	Status     JobStatus
	InputPath  string
	OutputPath string
	CreatedAt  time.Time
	UpdatedAt  *time.Time
}

type JobStatus string

func (j JobStatus) String() string {
    return string(j)
}

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)
