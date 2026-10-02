package main

import (
	"context"
	"fmt"

	"github.com/rafaelmgr12/goflow/internal/job"
)

func main() {
	ctx := context.Background()

	executor := job.NewExecutor()

	executor.Register("send_email", job.EmailHandler{})

	j := job.Job{
		ID:     "job-1",
		Type:   "send_email",
		Status: job.StatusPending,
	}

	if err := executor.Execute(ctx, j); err != nil {
		fmt.Println("failed to execute job:", err)
	}
}
