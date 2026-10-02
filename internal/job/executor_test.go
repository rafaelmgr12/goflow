package job

import (
	"context"
	"testing"
)

type fakeHandler struct {
	called bool
}

func (f *fakeHandler) Handle(ctx context.Context, job Job) error {
	f.called = true
	return nil
}

func TestExecutor_Execute(t *testing.T) {
	executor := NewExecutor()

	handler := &fakeHandler{}

	executor.Register("send_email", handler)

	j := Job{
		ID:   "job-1",
		Type: "send_email",
	}

	err := executor.Execute(context.Background(), j)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !handler.called {
		t.Fatal("expected handler to be called")
	}
}

func TestExecutor_ExecuteHandlerNotFound(t *testing.T) {
	executor := NewExecutor()

	j := Job{
		ID:   "job-1",
		Type: "send_sms",
	}

	err := executor.Execute(context.Background(), j)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
