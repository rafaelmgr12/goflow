package job

import (
	"context"
	"fmt"
)

type Executor struct {
	handlers map[string]Handler
}

func NewExecutor() *Executor {
	return &Executor{
		handlers: make(map[string]Handler),
	}
}

func (e *Executor) Register(jobType string, handler Handler) {
	e.handlers[jobType] = handler
}

func (e *Executor) Execute(ctx context.Context, job Job) error {
	handler, ok := e.handlers[job.Type]

	if !ok {
		return fmt.Errorf(
			"handler not found for job type %q",
			job.Type,
		)
	}

	return handler.Handle(ctx, job)
}
