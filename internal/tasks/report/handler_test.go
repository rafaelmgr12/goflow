package report

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type fakeWriter struct {
	calls    int
	document Document
	err      error
}

func (w *fakeWriter) Write(_ context.Context, document Document) error {
	w.calls++
	w.document = document
	return w.err
}

func TestHandlerHandleValidPayload(t *testing.T) {
	writer := &fakeWriter{}
	j := job.Job{ID: "report-123", Payload: []byte(`{"title":" Report title ","content":" Report body\nsecond line "}`)}
	if err := New(writer).Handle(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	want := Document{JobID: j.ID, Title: " Report title ", Content: " Report body\nsecond line "}
	if writer.calls != 1 || writer.document != want {
		t.Fatalf("writer received %d calls and %+v, want one call and %+v", writer.calls, writer.document, want)
	}
}

func TestHandlerHandleInvalidPayload(t *testing.T) {
	tests := []struct{ name, payload, message string }{
		{"invalid JSON", `{"title":`, "decoding report payload"},
		{"empty object", `{}`, "title"},
		{"null", `null`, "title"},
		{"empty title", `{"title":"","content":"body"}`, "title"},
		{"whitespace title", `{"title":" \t\n\u2003","content":"body"}`, "title"},
		{"missing content", `{"title":"title"}`, "content"},
		{"empty content", `{"title":"title","content":""}`, "content"},
		{"whitespace content", `{"title":"title","content":" \t\n\u2003"}`, "content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &fakeWriter{}
			err := New(writer).Handle(context.Background(), job.Job{ID: "report-123", Payload: []byte(tt.payload)})
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("expected error containing %q, got %v", tt.message, err)
			}
			if writer.calls != 0 {
				t.Fatalf("writer called %d times for invalid payload", writer.calls)
			}
			if tt.name == "invalid JSON" {
				var syntaxErr *json.SyntaxError
				if !errors.As(err, &syntaxErr) {
					t.Fatalf("expected wrapped JSON syntax error, got %v", err)
				}
			}
		})
	}
}

func TestHandlerHandleWriterError(t *testing.T) {
	wantErr := errors.New("writer failed")
	writer := &fakeWriter{err: wantErr}
	err := New(writer).Handle(context.Background(), job.Job{ID: "report-123", Payload: []byte(`{"title":"title","content":"body"}`)})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped writer error, got %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("writer called %d times, want 1", writer.calls)
	}
	if !strings.Contains(err.Error(), "writing report for job report-123") {
		t.Fatalf("missing error context: %v", err)
	}
}
