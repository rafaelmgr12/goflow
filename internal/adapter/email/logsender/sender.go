package logsender

import (
	"context"
	"log"

	"github.com/rafaelmgr12/goflow/internal/tasks/email"
)

type Sender struct{}

var _ email.Sender = (*Sender)(nil)

func New() *Sender {
	return &Sender{}
}

func (s *Sender) Send(ctx context.Context, message email.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	log.Printf("email: to=%q subject=%q body=%q", message.To, message.Subject, message.Body)
	return nil
}
