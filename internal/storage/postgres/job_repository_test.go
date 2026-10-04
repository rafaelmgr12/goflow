package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

	t.Cleanup(func() {
		db.Close()
	})

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging database: %v", err)
	}

	return db
}

func TestJobRepository_SaveAndFindByID(t *testing.T) {
	db := setupTestDB(t)

	repo := NewJobRepository(db)

	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)

	expected := job.Job{
		ID:          t.Name() + "-" + time.Now().UTC().Format("20060102150405.000000000"),
		Type:        "send_email",
		Payload:     []byte(`{"to":"rafa@example.com"}`),
		Status:      job.StatusPending,
		ScheduledAt: now.Add(time.Hour),
		CreatedAt:   now,
	}

	cleanupTestJob(t, db, expected.ID)

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

	if !actual.CreatedAt.Equal(expected.CreatedAt) {
		t.Fatalf("expected created_at %v, got %v", expected.CreatedAt, actual.CreatedAt)
	}
	if !actual.ScheduledAt.Equal(expected.ScheduledAt) {
		t.Fatalf("expected scheduled_at %v, got %v", expected.ScheduledAt, actual.ScheduledAt)
	}
	var payload map[string]string
	if err := json.Unmarshal(actual.Payload, &payload); err != nil {
		t.Fatalf("decoding payload: %v", err)
	}
	if payload["to"] != "rafa@example.com" || len(payload) != 1 {
		t.Fatalf("unexpected payload: %s", actual.Payload)
	}
}

func cleanupTestJob(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), "DELETE FROM jobs WHERE id = $1", id)
		if err != nil {
			t.Errorf("cleaning test job: %v", err)
		}
	})
}

func TestJobRepository_MarkRunning(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	for _, status := range []job.Status{
		job.StatusPending,
		job.StatusRunning,
		job.StatusCompleted,
		job.StatusFailed,
	} {
		t.Run(string(status), func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Microsecond)
			j := job.Job{
				ID:          t.Name() + "-" + time.Now().UTC().Format("20060102150405.000000000"),
				Type:        "send_email",
				Payload:     []byte(`{"to":"test@example.com"}`),
				Status:      status,
				CreatedAt:   now,
				ScheduledAt: now.Add(time.Hour),
			}
			cleanupTestJob(t, db, j.ID)
			if err := repo.Save(ctx, j); err != nil {
				t.Fatalf("saving job: %v", err)
			}

			err := repo.MarkRunning(ctx, j.ID)
			want := status
			if status == job.StatusPending {
				if err != nil {
					t.Fatalf("marking job as running: %v", err)
				}
				want = job.StatusRunning
			} else if !errors.Is(err, job.ErrInvalidTransition) {
				t.Fatalf("expected ErrInvalidTransition, got %v", err)
			}

			actual, err := repo.FindByID(ctx, j.ID)
			if err != nil {
				t.Fatalf("finding job: %v", err)
			}
			if actual.Status != want {
				t.Fatalf("expected status %s, got %s", want, actual.Status)
			}
			if actual.ID != j.ID || actual.Type != j.Type || !actual.CreatedAt.Equal(j.CreatedAt) || !actual.ScheduledAt.Equal(j.ScheduledAt) {
				t.Fatal("marking job as running changed other job fields")
			}
		})
	}
}

func TestJobRepository_MarkRunning_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()
	id := t.Name() + "-" + time.Now().UTC().Format("20060102150405.000000000")

	if err := repo.MarkRunning(ctx, id); !errors.Is(err, job.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
	if _, err := repo.FindByID(ctx, id); !errors.Is(err, job.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
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

func TestJobRepository_CompleteLifecycle(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)

	ctx := context.Background()
	now := time.Now().UTC()

	j := job.Job{
		ID:          fmt.Sprintf("job-%d", now.UnixNano()),
		Type:        "send_email",
		Payload:     []byte(`{}`),
		Status:      job.StatusPending,
		ScheduledAt: now,
		CreatedAt:   now,
	}

	t.Cleanup(func() {
		_, _ = db.Exec(
			"DELETE FROM jobs WHERE id = $1",
			j.ID,
		)
	})

	if err := repo.Save(ctx, j); err != nil {
		t.Fatalf("saving job: %v", err)
	}

	if err := repo.MarkRunning(ctx, j.ID); err != nil {
		t.Fatalf("marking running: %v", err)
	}

	if err := repo.MarkCompleted(ctx, j.ID); err != nil {
		t.Fatalf("marking completed: %v", err)
	}

	saved, err := repo.FindByID(ctx, j.ID)
	if err != nil {
		t.Fatalf("finding job: %v", err)
	}

	if saved.Status != job.StatusCompleted {
		t.Fatalf(
			"expected %s, got %s",
			job.StatusCompleted,
			saved.Status,
		)
	}
}
