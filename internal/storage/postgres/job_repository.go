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
