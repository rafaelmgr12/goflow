package scheduler

import (
	"context"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Repository interface {
	FindDueJobs(
		ctx context.Context,
		dueTime time.Time,
		limit int,
	) ([]job.Job, error)
}
