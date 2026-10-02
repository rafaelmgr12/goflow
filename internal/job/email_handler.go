package job

import (
	"context"
	"fmt"
)

type EmailHandler struct{}

var _ Handler = EmailHandler{}

func (h EmailHandler) Handle(ctx context.Context, j Job) error {
	fmt.Printf("sending email for job %s\n", j.ID)

	return nil
}
