package cmd

import (
	"context"
	"fmt"

	"github.com/rafaelmgr12/goflow/internal/job"
)

// TODO: Ask about struct and interface here, why we do not need to declare implicitly
type EmailHandler struct{}

func (h EmailHandler) Handle(ctx context.Context, j job.Job) error {
	fmt.Printf("sending email for job %s\n", j.ID)

	return nil
}
