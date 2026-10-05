package scheduler

import (
	"context"
	"fmt"
	"log"
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
) (*Scheduler, error) {

	if batchSize <= 0 {
		return nil, fmt.Errorf("batchSize must be greater than 0, got %d", batchSize)
	}

	if pollInterval <= 0 {
		return nil, fmt.Errorf("pollInterval must be greater than 0, got %v", pollInterval)
	}

	return &Scheduler{
		repository:   repository,
		jobs:         jobs,
		batchSize:    batchSize,
		pollInterval: pollInterval,
	}, nil
}

func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.poll(ctx, time.Now().UTC()); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		log.Printf("scheduler poll failed: %v", err)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := s.poll(ctx, time.Now().UTC()); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				log.Printf(
					"scheduler poll failed: %v",
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
