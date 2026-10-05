package scheduler

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type fakeRepository struct {
	called  bool
	calls   int
	ctx     context.Context
	dueTime time.Time
	limit   int
	jobs    []job.Job
	err     error
}

func (f *fakeRepository) FindDueJobs(ctx context.Context, dueTime time.Time, limit int) ([]job.Job, error) {
	f.called = true
	f.calls++
	f.ctx = ctx
	f.dueTime = dueTime
	f.limit = limit
	return f.jobs, f.err
}

func TestScheduler_Poll(t *testing.T) {
	repositoryErr := errors.New("repository unavailable")
	tests := []struct {
		name string
		jobs []job.Job
		err  error
	}{
		{
			name: "SendsDueJobs",
			jobs: []job.Job{
				{ID: "job-1", Type: "send_email", Status: job.StatusPending, Payload: []byte(`{}`)},
				{ID: "job-2", Type: "send_email", Status: job.StatusPending, Payload: []byte(`{}`)},
			},
		},
		{name: "NoDueJobs"},
		{name: "RepositoryError", err: repositoryErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			dueTime := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
			const batchSize = 10
			repository := &fakeRepository{
				jobs: tt.jobs,
				err:  tt.err,
			}
			// A buffer lets poll send without starting a worker goroutine.
			jobs := make(chan job.Job, batchSize)
			s := NewScheduler(repository, jobs, batchSize, time.Second)

			err := s.poll(ctx, dueTime)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected error %v, got %v", tt.err, err)
			}
			if !repository.called {
				t.Fatal("expected repository to be called")
			}
			if repository.calls != 1 {
				t.Fatalf("expected one repository call, got %d", repository.calls)
			}
			if repository.ctx != ctx || !repository.dueTime.Equal(dueTime) || repository.limit != batchSize {
				t.Fatal("unexpected repository arguments")
			}
			if len(jobs) != len(tt.jobs) {
				t.Fatalf("expected %d queued jobs, got %d", len(tt.jobs), len(jobs))
			}
			for _, want := range tt.jobs {
				got := <-jobs
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("expected job %+v, got %+v", want, got)
				}
			}
		})
	}
}

func TestScheduler_Poll_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repository := &fakeRepository{
		jobs: []job.Job{{ID: "job-1", Status: job.StatusPending}},
	}
	// With no receiver, sending cannot win the select against cancellation.
	jobs := make(chan job.Job)
	s := NewScheduler(repository, jobs, 10, time.Second)
	if err := s.poll(ctx, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
