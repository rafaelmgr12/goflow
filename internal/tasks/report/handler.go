package report

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Handler struct {
	writer Writer
}

func NewHandler(writer Writer) *Handler {
	return &Handler{
		writer: writer,
	}
}

var _ job.Handler = (*Handler)(nil)

func (h *Handler) Handle(ctx context.Context, j job.Job) error {
	var payload Payload

	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf(
			"decoding report payload: %w",
			err,
		)
	}

	document := Document{
		JobID:   j.ID,
		Title:   payload.Title,
		Content: payload.Content,
	}

	return h.writer.Write(ctx, document)
}
