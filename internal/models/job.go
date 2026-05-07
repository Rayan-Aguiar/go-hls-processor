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

func (j JobStatus) String() {
	panic("unimplemented")
}

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)
