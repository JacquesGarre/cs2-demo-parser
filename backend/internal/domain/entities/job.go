package entities

import "time"

type JobStatus string

const (
	JobStatusQueued     JobStatus = "QUEUED"
	JobStatusProcessing JobStatus = "PROCESSING"
	JobStatusCompleted  JobStatus = "COMPLETED"
	JobStatusFailed     JobStatus = "FAILED"
)

type AnalysisJob struct {
	ID        string
	DemoID    string
	Status    JobStatus
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
