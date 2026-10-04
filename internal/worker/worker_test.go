package worker

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type fakeHandler struct {
	called chan job.Job
}

func (f *fakeHandler) Handle(
	ctx context.Context,
	j job.Job,
) error {
	f.called <- j
	return nil
}

func TestWorkerProcessesJob(t *testing.T) {
	ctx := context.Background()

	handler := &fakeHandler{
		called: make(chan job.Job, 1),
	}

	executor := job.NewExecutor()

	executor.Register(
		"send_email",
		handler,
	)

	jobs := make(chan job.Job)

	done := make(chan struct{})

	go func() {
		Run(
			ctx,
			1,
			jobs,
			executor,
		)

		close(done)
	}()

	expectedJob := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	jobs <- expectedJob

	select {
	case receivedJob := <-handler.called:
		if receivedJob.ID != expectedJob.ID {
			t.Fatalf(
				"expected job %s, got %s",
				expectedJob.ID,
				receivedJob.ID,
			)
		}

	case <-time.After(time.Second):
		t.Fatal("worker did not process job")
	}

	close(jobs)

	select {
	case <-done:
		// worker finished

	case <-time.After(time.Second):
		t.Fatal("worker did not shut down")
	}
}

func TestWorkerStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	executor := job.NewExecutor()
	jobs := make(chan job.Job)

	done := make(chan struct{})

	go func() {
		Run(
			ctx,
			1,
			jobs,
			executor,
		)

		close(done)
	}()

	cancel()

	select {
	case <-done:
		// worker stopped

	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}
