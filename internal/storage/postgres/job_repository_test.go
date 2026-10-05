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

	"github.com/jackc/pgx/v5/pgconn"
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

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := fmt.Sprintf("test_jobs_%d", time.Now().UnixNano())
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("creating test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("cleaning test schema: %v", err)
		}
	})
	if _, err := db.ExecContext(ctx, "CREATE TABLE "+schema+".jobs (LIKE public.jobs INCLUDING ALL)"); err != nil {
		t.Fatalf("creating test jobs table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("setting test search_path: %v", err)
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

func TestJobRepository_MarkRunning(t *testing.T) {
	testStatusTransition(t, (*JobRepository).MarkRunning, job.StatusPending, job.StatusRunning)
}

func TestJobRepository_MarkCompleted(t *testing.T) {
	testStatusTransition(t, (*JobRepository).MarkCompleted, job.StatusRunning, job.StatusCompleted)
}

func TestJobRepository_MarkFailed(t *testing.T) {
	testStatusTransition(t, (*JobRepository).MarkFailed, job.StatusRunning, job.StatusFailed)
}

func testStatusTransition(
	t *testing.T,
	mark func(*JobRepository, context.Context, string) error,
	from job.Status,
	to job.Status,
) {
	t.Helper()
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
			if err := repo.Save(ctx, j); err != nil {
				t.Fatalf("saving job: %v", err)
			}

			err := mark(repo, ctx, j.ID)
			want := status
			if status == from {
				if err != nil {
					t.Fatalf("marking job as %s: %v", to, err)
				}
				want = to
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
				t.Fatal("status transition changed other job fields")
			}
		})
	}
}

func TestJobRepository_StatusCheck(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	j := job.Job{
		ID:   t.Name() + "-" + now.Format("20060102150405.000000000"),
		Type: "send_email", Payload: []byte(`{}`),
		Status: job.Status("banana"), CreatedAt: now, ScheduledAt: now,
	}

	t.Run("Insert", func(t *testing.T) {
		err := repo.Save(ctx, j)
		assertStatusCheckViolation(t, err)
		if _, err := repo.FindByID(ctx, j.ID); !errors.Is(err, job.ErrNotFound) {
			t.Fatalf("expected rejected job to be absent, got %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		j.Status = job.StatusPending
		if err := repo.Save(ctx, j); err != nil {
			t.Fatalf("saving job: %v", err)
		}
		_, err := db.ExecContext(ctx, "UPDATE jobs SET status = $1 WHERE id = $2", "banana", j.ID)
		assertStatusCheckViolation(t, err)
		actual, err := repo.FindByID(ctx, j.ID)
		if err != nil {
			t.Fatalf("finding job: %v", err)
		}
		if actual.Status != job.StatusPending {
			t.Fatalf("expected status pending, got %s", actual.Status)
		}
	})
}

func assertStatusCheckViolation(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "jobs_status_check" {
		t.Fatalf("expected jobs_status_check violation, got %v", err)
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

func TestJobRepository_FindDueJobs(t *testing.T) {
	tests := []struct {
		status    job.Status
		wantCount int
	}{
		{status: job.StatusPending, wantCount: 1},
		{status: job.StatusRunning, wantCount: 0},
		{status: job.StatusCompleted, wantCount: 0},
		{status: job.StatusFailed, wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			db := setupTestDB(t)
			repo := NewJobRepository(db)
			ctx := context.Background()
			now := time.Now().UTC().Truncate(time.Microsecond)
			j := job.Job{
				ID:          "integration-due-job",
				Type:        "send_email",
				Payload:     []byte(`{"to":"test@example.com"}`),
				Status:      tt.status,
				CreatedAt:   now.Add(-2 * time.Hour),
				ScheduledAt: now.Add(-time.Hour),
			}

			if err := repo.Save(ctx, j); err != nil {
				t.Fatalf("saving job: %v", err)
			}

			actual, err := repo.FindDueJobs(ctx, now, 10)
			if err != nil {
				t.Fatalf("finding due jobs: %v", err)
			}
			if len(actual) != tt.wantCount {
				t.Fatalf("expected %d jobs, got %d", tt.wantCount, len(actual))
			}
			if tt.wantCount == 1 {
				if actual[0].ID != j.ID {
					t.Fatalf("expected id %s, got %s", j.ID, actual[0].ID)
				}
				if actual[0].Status != job.StatusPending {
					t.Fatalf("expected status pending, got %s", actual[0].Status)
				}
			}
		})
	}
}

func TestJobRepository_FindDueJobs_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	jobs, err := repo.FindDueJobs(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("finding due jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %v", jobs)
	}
}

func TestJobRepository_FindDueJobs_Contract(t *testing.T) {
	dueTime := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		jobs    []job.Job
		limit   int
		wantIDs []string
	}{
		{
			name: "Time",
			jobs: []job.Job{
				{ID: "C", ScheduledAt: dueTime.Add(time.Minute)},
				{ID: "B", ScheduledAt: dueTime},
				{ID: "A", ScheduledAt: dueTime.Add(-time.Hour)},
			},
			limit:   10,
			wantIDs: []string{"A", "B"},
		},
		{
			name: "PositiveLimit",
			jobs: []job.Job{
				{ID: "E", ScheduledAt: dueTime},
				{ID: "D", ScheduledAt: dueTime.Add(-time.Minute)},
				{ID: "C", ScheduledAt: dueTime.Add(-2 * time.Minute)},
				{ID: "B", ScheduledAt: dueTime.Add(-3 * time.Minute)},
				{ID: "A", ScheduledAt: dueTime.Add(-4 * time.Minute)},
			},
			limit:   2,
			wantIDs: []string{"A", "B"},
		},
		{
			name: "Ordering",
			// Reverse the insertion order, with ties on scheduled_at and created_at.
			jobs: []job.Job{
				{ID: "D", ScheduledAt: dueTime, CreatedAt: dueTime.Add(-2 * time.Hour)},
				{ID: "A", ScheduledAt: dueTime, CreatedAt: dueTime.Add(-2 * time.Hour)},
				{ID: "C", ScheduledAt: dueTime, CreatedAt: dueTime.Add(-3 * time.Hour)},
				{ID: "B", ScheduledAt: dueTime.Add(-time.Hour), CreatedAt: dueTime.Add(-90 * time.Minute)},
			},
			limit:   10,
			wantIDs: []string{"B", "C", "A", "D"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			repo := NewJobRepository(db)
			ctx := context.Background()
			for _, j := range tt.jobs {
				j.Type = "send_email"
				j.Payload = []byte(`{}`)
				j.Status = job.StatusPending
				if j.CreatedAt.IsZero() {
					j.CreatedAt = dueTime.Add(-24 * time.Hour)
				}
				if err := repo.Save(ctx, j); err != nil {
					t.Fatalf("saving job %s: %v", j.ID, err)
				}
			}

			actual, err := repo.FindDueJobs(ctx, dueTime, tt.limit)
			if err != nil {
				t.Fatalf("finding due jobs: %v", err)
			}
			if len(actual) != len(tt.wantIDs) {
				t.Fatalf("expected %d jobs, got %d", len(tt.wantIDs), len(actual))
			}
			for i, id := range tt.wantIDs {
				if actual[i].ID != id {
					t.Fatalf("expected id %s at index %d, got %s", id, i, actual[i].ID)
				}
			}
		})
	}
}

func TestJobRepository_FindDueJobs_InvalidLimit(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	for _, limit := range []int{0, -1} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			jobs, err := repo.FindDueJobs(context.Background(), time.Now().UTC(), limit)
			if err == nil {
				t.Fatal("expected invalid limit to fail")
			}
			if jobs != nil {
				t.Fatalf("expected no jobs on error, got %v", jobs)
			}
		})
	}
}

func TestJobRepository_FindDueJobs_CanceledContext(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	jobs, err := repo.FindDueJobs(ctx, time.Now().UTC(), 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if jobs != nil {
		t.Fatalf("expected no jobs on error, got %v", jobs)
	}
}
