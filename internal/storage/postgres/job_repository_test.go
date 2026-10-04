package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rafaelmgr12/goflow/internal/job"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")

	if databaseURL == "" {
		databaseURL = "postgres://goflow:goflow@localhost:5432/goflow?sslmode=disable"
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestJobRepository_SaveAndFindByID(t *testing.T) {
	db := setupTestDB(t)

	repo := NewJobRepository(db)

	ctx := context.Background()

	now := time.Now().UTC()

	expected := job.Job{
		ID:          "integration-job-1",
		Type:        "send_email",
		Payload:     []byte(`{"to":"rafa@example.com"}`),
		Status:      job.StatusPending,
		ScheduledAt: now,
		CreatedAt:   now,
	}

	_, err := db.ExecContext(
		ctx,
		"DELETE FROM jobs WHERE id = $1",
		expected.ID,
	)
	if err != nil {
		t.Fatalf("cleaning test job: %v", err)
	}

	if err := repo.Save(ctx, expected); err != nil {
		t.Fatalf("saving job: %v", err)
	}

	actual, err := repo.FindByID(ctx, expected.ID)
	if err != nil {
		t.Fatalf("finding job: %v", err)
	}

	if actual.ID != expected.ID {
		t.Fatalf(
			"expected id %s, got %s",
			expected.ID,
			actual.ID,
		)
	}

	if actual.Type != expected.Type {
		t.Fatalf(
			"expected type %s, got %s",
			expected.Type,
			actual.Type,
		)
	}

	if actual.Status != expected.Status {
		t.Fatalf(
			"expected status %s, got %s",
			expected.Status,
			actual.Status,
		)
	}

	t.Cleanup(func() {
		_, err := db.ExecContext(
			context.Background(),
			"DELETE FROM jobs WHERE id = $1",
			expected.ID,
		)
		if err != nil {
			t.Errorf("cleaning test job: %v", err)
		}
	})
}

func TestJobRepository_FindByID_NotFound(t *testing.T) {
	db := setupTestDB(t)

	repo := NewJobRepository(db)

	ctx := context.Background()

	_, err := repo.FindByID(
		ctx,
		"job-that-does-not-exist",
	)

	if !errors.Is(err, job.ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}
