package job

import (
	"errors"
	"testing"
)

func TestJob_Fail(t *testing.T) {
	for _, status := range []Status{StatusPending, StatusRunning, StatusCompleted, StatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			j := Job{Status: status}
			err := j.Fail()
			want := status
			if status == StatusRunning {
				if err != nil {
					t.Fatalf("failing job: %v", err)
				}
				want = StatusFailed
			} else if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected ErrInvalidTransition, got %v", err)
			}
			if j.Status != want {
				t.Fatalf("expected status %s, got %s", want, j.Status)
			}
		})
	}
}

func TestJob_Start(t *testing.T) {
	j := Job{
		Status: StatusPending,
	}

	if err := j.Start(); err != nil {
		t.Fatalf("starting job: %v", err)
	}

	if j.Status != StatusRunning {
		t.Fatalf(
			"expected status %s, got %s",
			StatusRunning,
			j.Status,
		)
	}
}

func TestJob_Start_InvalidTransition(t *testing.T) {
	j := Job{
		Status: StatusCompleted,
	}

	err := j.Start()

	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf(
			"expected ErrInvalidTransition, got %v",
			err,
		)
	}
}

func TestJob_Complete(t *testing.T) {
	j := Job{
		Status: StatusRunning,
	}

	if err := j.Complete(); err != nil {
		t.Fatalf("completing job: %v", err)
	}

	if j.Status != StatusCompleted {
		t.Fatalf(
			"expected %s, got %s",
			StatusCompleted,
			j.Status,
		)
	}
}

func TestJob_Complete_InvalidTransition(t *testing.T) {
	j := Job{
		Status: StatusPending,
	}

	err := j.Complete()

	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf(
			"expected ErrInvalidTransition, got %v",
			err,
		)
	}
}
