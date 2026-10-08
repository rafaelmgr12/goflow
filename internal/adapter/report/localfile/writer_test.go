package localfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rafaelmgr12/goflow/internal/tasks/report"
)

func TestWriterWrite(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing directory", true: "create nested directory"}[nested], func(t *testing.T) {
			directory := t.TempDir()
			if nested {
				directory = filepath.Join(directory, "reports", "output")
			}
			document := report.Document{JobID: "report-123", Title: "Relatório GoFlow", Content: "First line\nSecond line"}
			if err := New(directory).Write(context.Background(), document); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(directory, "report-123.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "Relatório GoFlow\n\nFirst line\nSecond line\n" {
				t.Fatalf("unexpected content: %q", content)
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "report-123.txt" {
				t.Fatalf("unexpected output files: %v", entries)
			}
		})
	}
}

func TestWriterWriteInvalidJobID(t *testing.T) {
	for _, id := range []string{"", ".", "..", "../escape", "../../escape", "nested/report", "/absolute", `..\escape`, `nested\report`, `C:report`, "nul\x00report"} {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "output")
			err := New(directory).Write(context.Background(), report.Document{JobID: id, Title: "title", Content: "body"})
			if err == nil || !strings.Contains(err.Error(), "invalid report job ID") {
				t.Fatalf("expected invalid ID error, got %v", err)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid ID created filesystem entries: %v", entries)
			}
		})
	}
}

func TestWriterWriteCanceledContext(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "output")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := New(directory).Write(ctx, report.Document{JobID: "report-123", Title: "title", Content: "body"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no output directory, got %v", err)
	}
}

func TestWriterWriteFilesystemErrors(t *testing.T) {
	for _, operation := range []string{"creating report directory", "writing report"} {
		t.Run(operation, func(t *testing.T) {
			directory := t.TempDir()
			var blockedPath string
			if operation == "creating report directory" {
				blockedPath = filepath.Join(directory, "blocked")
				if err := os.WriteFile(blockedPath, []byte("file"), 0600); err != nil {
					t.Fatal(err)
				}
				directory = blockedPath
			} else {
				blockedPath = filepath.Join(directory, "report-123.txt")
				if err := os.Mkdir(blockedPath, 0755); err != nil {
					t.Fatal(err)
				}
			}
			err := New(directory).Write(context.Background(), report.Document{JobID: "report-123", Title: "title", Content: "body"})
			if err == nil || !strings.Contains(err.Error(), operation) {
				t.Fatalf("expected %s error, got %v", operation, err)
			}
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("expected wrapped filesystem error, got %v", err)
			}
			if !errors.Is(err, pathErr.Err) {
				t.Fatalf("underlying filesystem error was not preserved: %v", err)
			}
		})
	}
}

func TestWriterWriteOverwritesExistingFile(t *testing.T) {
	directory := t.TempDir()
	writer := New(directory)
	document := report.Document{JobID: "report-123", Title: "Original title", Content: "A longer original body"}
	if err := writer.Write(context.Background(), document); err != nil {
		t.Fatal(err)
	}
	document.Title, document.Content = "New", "Body"
	if err := writer.Write(context.Background(), document); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(directory, "report-123.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "New\n\nBody\n" {
		t.Fatalf("expected overwritten and truncated file, got %q", content)
	}
}
