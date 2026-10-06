package processor

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type fakeRepository struct {
	calls            *[]string
	markRunningErr   error
	markCompletedErr error
	markFailedErr    error
}

func (f *fakeRepository) MarkRunning(
	ctx context.Context,
	id string,
) error {
	*f.calls = append(*f.calls, "MarkRunning:"+id)
	return f.markRunningErr
}

func (f *fakeRepository) MarkCompleted(
	ctx context.Context,
	id string,
) error {
	*f.calls = append(*f.calls, "MarkCompleted:"+id)
	return f.markCompletedErr
}

func (f *fakeRepository) MarkFailed(
	ctx context.Context,
	id string,
) error {
	*f.calls = append(*f.calls, "MarkFailed:"+id)
	return f.markFailedErr
}

type fakeExecutor struct {
	calls *[]string
	err   error
}

func (f *fakeExecutor) Execute(
	ctx context.Context,
	j job.Job,
) error {
	*f.calls = append(*f.calls, "Execute:"+j.ID)
	return f.err
}

func TestProcessor_Process_Success(t *testing.T) {
	calls := []string{}

	repo := &fakeRepository{
		calls: &calls,
	}

	executor := &fakeExecutor{
		calls: &calls,
	}

	p := NewProcessor(repo, executor)

	j := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	err := p.Process(context.Background(), j)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wantCalls := []string{
		"MarkRunning:job-1",
		"Execute:job-1",
		"MarkCompleted:job-1",
	}

	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf(
			"expected calls %v, got %v",
			wantCalls,
			calls,
		)
	}
}

func TestProcessor_Process_ExecutionFails(t *testing.T) {
	calls := []string{}
	executorErr := errors.New("execution failed")

	repo := &fakeRepository{
		calls: &calls,
	}
	executor := &fakeExecutor{
		calls: &calls,
		err:   executorErr,
	}
	p := NewProcessor(repo, executor)
	j := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	err := p.Process(context.Background(), j)
	if !errors.Is(err, executorErr) {
		t.Fatalf("expected error %v, got %v", executorErr, err)
	}

	wantCalls := []string{
		"MarkRunning:job-1",
		"Execute:job-1",
		"MarkFailed:job-1",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("expected calls %v, got %v", wantCalls, calls)
	}
}

func TestProcessor_Process_MarkRunningInvalidTransition(t *testing.T) {
	calls := []string{}

	repo := &fakeRepository{
		calls:          &calls,
		markRunningErr: job.ErrInvalidTransition,
	}
	executor := &fakeExecutor{
		calls: &calls,
	}
	p := NewProcessor(repo, executor)
	j := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	err := p.Process(context.Background(), j)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wantCalls := []string{
		"MarkRunning:job-1",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("expected calls %v, got %v", wantCalls, calls)
	}
}

func TestProcessor_Process_MarkRunningFails(t *testing.T) {
	calls := []string{}
	repositoryErr := errors.New("database unavailable")

	repo := &fakeRepository{
		calls:          &calls,
		markRunningErr: repositoryErr,
	}
	executor := &fakeExecutor{
		calls: &calls,
	}
	p := NewProcessor(repo, executor)
	j := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	err := p.Process(context.Background(), j)
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("expected error %v, got %v", repositoryErr, err)
	}

	wantCalls := []string{
		"MarkRunning:job-1",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("expected calls %v, got %v", wantCalls, calls)
	}
}

func TestProcessor_Process_MarkCompletedFails(t *testing.T) {
	calls := []string{}
	repositoryErr := errors.New("database unavailable")

	repo := &fakeRepository{
		calls:            &calls,
		markCompletedErr: repositoryErr,
	}
	executor := &fakeExecutor{
		calls: &calls,
	}
	p := NewProcessor(repo, executor)
	j := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	err := p.Process(context.Background(), j)
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("expected error %v, got %v", repositoryErr, err)
	}

	wantCalls := []string{
		"MarkRunning:job-1",
		"Execute:job-1",
		"MarkCompleted:job-1",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("expected calls %v, got %v", wantCalls, calls)
	}
}

func TestProcessor_Process_ExecutionAndMarkFailedFail(t *testing.T) {
	calls := []string{}
	executorErr := errors.New("execution failed")
	repositoryErr := errors.New("database unavailable")

	repo := &fakeRepository{
		calls:         &calls,
		markFailedErr: repositoryErr,
	}
	executor := &fakeExecutor{
		calls: &calls,
		err:   executorErr,
	}
	p := NewProcessor(repo, executor)
	j := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	err := p.Process(context.Background(), j)
	if !errors.Is(err, executorErr) {
		t.Fatalf("expected error %v, got %v", executorErr, err)
	}
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("expected error %v, got %v", repositoryErr, err)
	}

	wantCalls := []string{
		"MarkRunning:job-1",
		"Execute:job-1",
		"MarkFailed:job-1",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("expected calls %v, got %v", wantCalls, calls)
	}
}
