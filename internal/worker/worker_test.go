package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type fakeProcessor struct {
	processed chan job.Job
	err       error
}

func (f *fakeProcessor) Process(
	ctx context.Context,
	j job.Job,
) error {
	if f.processed != nil {
		f.processed <- j
	}

	return f.err
}

func TestWorkerProcessesJob(t *testing.T) {
	ctx := context.Background()

	processor := &fakeProcessor{
		processed: make(chan job.Job, 1),
	}

	jobs := make(chan job.Job)
	done := make(chan struct{})

	go func() {
		Run(
			ctx,
			1,
			jobs,
			processor,
			30*time.Second,
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
	case receivedJob := <-processor.processed:
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
	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	processor := &fakeProcessor{}

	jobs := make(chan job.Job)
	done := make(chan struct{})

	go func() {
		Run(
			ctx,
			1,
			jobs,
			processor,
			30*time.Second,
		)

		close(done)
	}()

	cancel()

	select {
	case <-done:
		// worker stopped

	case <-time.After(time.Second):
		t.Fatal(
			"worker did not stop after context cancellation",
		)
	}
}

type blockingProcessor struct {
	started chan context.Context
	release chan struct{}
}

func (p *blockingProcessor) Process(
	ctx context.Context,
	j job.Job,
) error {
	p.started <- ctx

	<-p.release

	return nil
}

func TestWorkerAllowsInFlightJobToFinishAfterCancellation(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	processor := &blockingProcessor{
		started: make(chan context.Context, 1),
		release: make(chan struct{}),
	}

	jobs := make(chan job.Job)
	done := make(chan struct{})

	go func() {
		Run(
			ctx,
			1,
			jobs,
			processor,
			30*time.Second,
		)

		close(done)
	}()

	jobs <- job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	var jobCtx context.Context

	select {
	case jobCtx = <-processor.started:

	case <-time.After(time.Second):
		t.Fatal("worker did not start processing job")
	}

	cancel()

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf(
			"expected worker context to be canceled, got %v",
			ctx.Err(),
		)
	}

	if err := jobCtx.Err(); err != nil {
		t.Fatalf(
			"expected in-flight job context to remain active, got %v",
			err,
		)
	}

	close(processor.release)

	select {
	case <-done:
		// worker finished after the in-flight job completed

	case <-time.After(time.Second):
		t.Fatal(
			"worker did not shut down after in-flight job completed",
		)
	}
}

type timeoutProcessor struct {
	result chan error
}

func (p *timeoutProcessor) Process(ctx context.Context, _ job.Job) error {
	<-ctx.Done()
	p.result <- ctx.Err()
	return ctx.Err()
}

func TestWorkerUsesConfiguredJobTimeout(t *testing.T) {
	processor := &timeoutProcessor{result: make(chan error, 1)}
	jobs := make(chan job.Job, 1)
	jobs <- job.Job{ID: "timeout-job"}
	close(jobs)
	done := make(chan struct{})
	go func() {
		Run(context.Background(), 1, jobs, processor, 10*time.Millisecond)
		close(done)
	}()
	select {
	case err := <-processor.result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not enforce configured timeout")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not finish")
	}
}
