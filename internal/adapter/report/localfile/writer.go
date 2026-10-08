package localfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rafaelmgr12/goflow/internal/tasks/report"
)

type Writer struct {
	directory string
}

var _ report.Writer = (*Writer)(nil)

func New(directory string) *Writer {
	return &Writer{directory: directory}
}

func (w *Writer) Write(ctx context.Context, document report.Document) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if document.JobID == "" || document.JobID == "." || document.JobID == ".." || strings.ContainsAny(document.JobID, "/\\\x00:") {
		return fmt.Errorf("invalid report job ID: %q", document.JobID)
	}
	if err := os.MkdirAll(w.directory, 0755); err != nil {
		return fmt.Errorf("creating report directory: %w", err)
	}
	path := filepath.Join(w.directory, document.JobID+".txt")
	content := document.Title + "\n\n" + document.Content + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	return nil
}
