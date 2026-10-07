package email

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Handler struct {
	sender Sender
}

func NewHandler(sender Sender) *Handler {
	return &Handler{
		sender: sender,
	}
}

var _ job.Handler = (*Handler)(nil)

func (h *Handler) Handle(ctx context.Context, j job.Job) error {
	var payload Payload

	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf(
			"decoding email payload: %w",
			err,
		)
	}

	return h.sender.Send(
		ctx,
		Message{
			To:      payload.To,
			Subject: payload.Subject,
			Body:    payload.Body,
		},
	)
}
