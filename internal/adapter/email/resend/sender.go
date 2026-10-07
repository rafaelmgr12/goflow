package resend

import (
	"context"
	"fmt"

	resendgo "github.com/resend/resend-go/v4"

	"github.com/rafaelmgr12/goflow/internal/tasks/email"
)

type Sender struct {
	client *resendgo.Client
	from   string
}

var _ email.Sender = (*Sender)(nil)

func New(
	apiKey string,
	from string,
) *Sender {
	return &Sender{
		client: resendgo.NewClient(apiKey),
		from:   from,
	}
}

func (s *Sender) Send(
	ctx context.Context,
	message email.Message,
) error {
	params := &resendgo.SendEmailRequest{
		From:    s.from,
		To:      []string{message.To},
		Subject: message.Subject,
		Html:    message.Body,
	}

	_, err := s.client.Emails.SendWithContext(
		ctx,
		params,
	)
	if err != nil {
		return fmt.Errorf(
			"sending email with resend: %w",
			err,
		)
	}

	return nil
}
