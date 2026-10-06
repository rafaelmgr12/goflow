package processor

import (
	"context"
	"errors"
	"fmt"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Processor struct {
	repo     Repository
	executor Executor
}

func NewProcessor(
	repo Repository,
	executor Executor,
) *Processor {
	return &Processor{
		repo:     repo,
		executor: executor,
	}
}

func (p *Processor) Process(
	ctx context.Context,
	j job.Job,
) error {
	err := p.repo.MarkRunning(ctx, j.ID)
	if err != nil {
		if errors.Is(err, job.ErrInvalidTransition) {
			return nil
		}

		return fmt.Errorf(
			"marking job %s as running: %w",
			j.ID,
			err,
		)
	}

	if err := p.executor.Execute(ctx, j); err != nil {
		if markErr := p.repo.MarkFailed(ctx, j.ID); markErr != nil {
			return errors.Join(
				err,
				fmt.Errorf(
					"marking job %s as failed: %w",
					j.ID,
					markErr,
				),
			)
		}

		return err
	}

	if err := p.repo.MarkCompleted(ctx, j.ID); err != nil {
		return fmt.Errorf(
			"marking job %s as completed: %w",
			j.ID,
			err,
		)
	}

	return nil
}
