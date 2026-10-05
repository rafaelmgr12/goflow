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
	errors  []error
	queried chan time.Time
}

func (f *fakeRepository) FindDueJobs(ctx context.Context, dueTime time.Time, limit int) ([]job.Job, error) {
	f.called = true
	f.calls++
	f.ctx = ctx
	f.dueTime = dueTime
	f.limit = limit
	if f.queried != nil {
		select {
		case f.queried <- dueTime:
		default:
		}
	}
	if f.calls <= len(f.errors) {
		return nil, f.errors[f.calls-1]
	}
	return f.jobs, f.err
}

func TestScheduler_Run_PollsImmediately(t *testing.T) {
	repository := &fakeRepository{queried: make(chan time.Time, 1)}
	s := NewScheduler(repository, make(chan job.Job), 10, time.Hour)
	done, cancel := startScheduler(t, s)

	select {
	case dueTime := <-repository.queried:
		if dueTime.Location() != time.UTC {
			t.Fatalf("expected UTC dueTime, got %v", dueTime.Location())
		}
	case err := <-done:
		t.Fatalf("scheduler stopped before polling: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("expected polling before the first hourly tick")
	}
	cancel()
	assertSchedulerCanceled(t, done)
}

func TestScheduler_Run_ContinuesAfterPollError(t *testing.T) {
	repositoryErr := errors.New("repository unavailable")
	tests := []struct {
		name   string
		errors []error
	}{
		{name: "InitialPollError", errors: []error{repositoryErr}},
		{name: "PeriodicPollError", errors: []error{nil, repositoryErr}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := job.Job{ID: "job-1", Status: job.StatusPending}
			repository := &fakeRepository{errors: tt.errors, jobs: []job.Job{want}}
			jobs := make(chan job.Job)
			s := NewScheduler(repository, jobs, 10, 10*time.Millisecond)
			done, cancel := startScheduler(t, s)

			select {
			case got := <-jobs:
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("expected job %+v, got %+v", want, got)
				}
			case err := <-done:
				t.Fatalf("scheduler stopped instead of retrying: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("expected a job from the poll after the error")
			}
			cancel()
			assertSchedulerCanceled(t, done)
		})
	}
}

func TestScheduler_Run_CanceledWhileSending(t *testing.T) {
	repository := &fakeRepository{
		jobs:    []job.Job{{ID: "job-1", Status: job.StatusPending}},
		queried: make(chan time.Time, 1),
	}
	// No receiver: Run must leave the blocked send when canceled.
	s := NewScheduler(repository, make(chan job.Job), 10, time.Hour)
	done, cancel := startScheduler(t, s)
	select {
	case <-repository.queried:
	case err := <-done:
		t.Fatalf("scheduler stopped before polling: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("expected repository query")
	}
	cancel()
	assertSchedulerCanceled(t, done)
}

func TestScheduler_Run_CanceledContextWithRepositoryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repository := &fakeRepository{err: errors.New("repository unavailable")}
	s := NewScheduler(repository, make(chan job.Job), 10, time.Hour)
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func startScheduler(t *testing.T, s *Scheduler) (<-chan error, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		done <- s.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Error("scheduler did not stop after cancellation")
		}
	})
	return done, cancel
}

func assertSchedulerCanceled(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not return after cancellation")
	}
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
