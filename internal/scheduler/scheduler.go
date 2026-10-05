package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Scheduler struct {
	repository   Repository
	jobs         chan<- job.Job
	batchSize    int
	pollInterval time.Duration
}

func NewScheduler(
	repository Repository,
	jobs chan<- job.Job,
	batchSize int,
	pollInterval time.Duration,
) *Scheduler {
	return &Scheduler{
		repository:   repository,
		jobs:         jobs,
		batchSize:    batchSize,
		pollInterval: pollInterval,
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := s.poll(ctx, time.Now()); err != nil {
				return fmt.Errorf(
					"polling for due jobs: %w",
					err,
				)
			}
		}
	}
}

func (s *Scheduler) poll(
	ctx context.Context,
	dueTime time.Time,
) error {
	jobs, err := s.repository.FindDueJobs(
		ctx,
		dueTime,
		s.batchSize,
	)
	if err != nil {
		return fmt.Errorf(
			"finding due jobs: %w",
			err,
		)
	}

	for _, j := range jobs {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case s.jobs <- j:
		}
	}

	return nil
}
