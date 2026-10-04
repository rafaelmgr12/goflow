package job

import (
	"errors"
	"time"
)

type Status string

var ErrInvalidTransition = errors.New("invalid job status transition")

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

func (j *Job) Start() error {
	if j.Status != StatusPending {
		return ErrInvalidTransition
	}

	j.Status = StatusRunning

	return nil
}

func (j *Job) Complete() error {
	if j.Status != StatusRunning {
		return ErrInvalidTransition
	}

	j.Status = StatusCompleted

	return nil
}

func (j *Job) Fail() error {
	if j.Status != StatusRunning {
		return ErrInvalidTransition
	}

	j.Status = StatusFailed
	return nil
}
