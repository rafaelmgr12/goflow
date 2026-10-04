package job

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID          string
	Type        string
	Payload     []byte
	Status      Status
	CreatedAt   time.Time
	ScheduledAt time.Time
}
