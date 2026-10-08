package report

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Handler struct {
	writer Writer
}

func New(writer Writer) *Handler {
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

	if strings.TrimSpace(payload.Title) == "" {
		return fmt.Errorf("validating report payload: title must not be empty or whitespace")
	}
	if strings.TrimSpace(payload.Content) == "" {
		return fmt.Errorf("validating report payload: content must not be empty or whitespace")
	}

	document := Document{
		JobID:   j.ID,
		Title:   payload.Title,
		Content: payload.Content,
	}

	if err := h.writer.Write(ctx, document); err != nil {
		return fmt.Errorf("writing report for job %s: %w", j.ID, err)
	}
	return nil
}
