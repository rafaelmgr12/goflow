package job

import (
	"context"
	"log"
	"time"
)

type EmailHandler struct{}

var _ Handler = EmailHandler{}

func (h EmailHandler) Handle(
	ctx context.Context,
	j Job,
) error {
	log.Printf("sending email for job %s", j.ID)

	time.Sleep(2 * time.Second)

	log.Printf("email sent for job %s", j.ID)

	return nil
}
