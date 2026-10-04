package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rafaelmgr12/goflow/internal/job"
)

var _ job.Repository = (*JobRepository)(nil)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{
		db: db,
	}
}

func (r *JobRepository) Save(
	ctx context.Context,
	j job.Job,
) error {
	const query = `
		INSERT INTO jobs (
			id,
			type,
			payload,
			status,
			created_at,
			scheduled_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		j.ID,
		j.Type,
		j.Payload,
		j.Status,
		j.CreatedAt,
		j.ScheduledAt,
	)

	if err != nil {
		return fmt.Errorf("saving job %s: %w", j.ID, err)
	}

	return nil
}

func (r *JobRepository) FindByID(
	ctx context.Context,
	id string,
) (job.Job, error) {
	const query = `
		SELECT
			id,
			type,
			payload,
			status,
			scheduled_at,
			created_at
		FROM jobs
		WHERE id = $1
	`

	var j job.Job

	err := r.db.QueryRowContext(
		ctx,
		query,
		id,
	).Scan(
		&j.ID,
		&j.Type,
		&j.Payload,
		&j.Status,
		&j.ScheduledAt,
		&j.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return job.Job{}, job.ErrNotFound
	}

	if err != nil {
		return job.Job{}, fmt.Errorf(
			"finding job %s: %w",
			id,
			err,
		)
	}

	return j, nil
}

func (r *JobRepository) MarkRunning(
	ctx context.Context,
	id string,
) error {
	const query = `
		UPDATE jobs
		SET status = $1
		WHERE id = $2
		  AND status = $3
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		job.StatusRunning,
		id,
		job.StatusPending,
	)
	if err != nil {
		return fmt.Errorf(
			"marking job %s as running: %w",
			id,
			err,
		)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"getting affected rows for job %s: %w",
			id,
			err,
		)
	}

	if rows == 0 {
		return job.ErrInvalidTransition
	}

	return nil
}

func (r *JobRepository) MarkCompleted(
	ctx context.Context,
	id string,
) error {
	const query = `
		UPDATE jobs
		SET status = $1
		WHERE id = $2
		  AND status = $3
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		job.StatusCompleted,
		id,
		job.StatusRunning,
	)
	if err != nil {
		return fmt.Errorf(
			"marking job %s as completed: %w",
			id,
			err,
		)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"getting affected rows for job %s: %w",
			id,
			err,
		)
	}

	if rows == 0 {
		return job.ErrInvalidTransition
	}

	return nil
}

func (r *JobRepository) MarkFailed(
	ctx context.Context,
	id string,
) error {
	const query = `
		UPDATE jobs
		SET status = $1
		WHERE id = $2
		  AND status = $3
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		job.StatusFailed,
		id,
		job.StatusRunning,
	)
	if err != nil {
		return fmt.Errorf(
			"marking job %s as failed: %w",
			id,
			err,
		)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"getting affected rows for job %s: %w",
			id,
			err,
		)
	}

	if rows == 0 {
		return job.ErrInvalidTransition
	}

	return nil
}
