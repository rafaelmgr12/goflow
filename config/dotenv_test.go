package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadDotEnv(t *testing.T) {
	tests := []struct {
		name, content string
		process       map[string]string
		wantCount     int
		wantErr       string
	}{
		{name: "file settings", content: "WORKER_COUNT=8\nWORKER_JOB_TIMEOUT=2m\n", wantCount: 8},
		{name: "process takes precedence", content: "WORKER_COUNT=8\n", process: map[string]string{"WORKER_COUNT": "4"}, wantCount: 4},
		{name: "empty process value takes precedence", content: "WORKER_COUNT=8\n", process: map[string]string{"WORKER_COUNT": ""}, wantErr: "WORKER_COUNT"},
		{name: "comments and quotes", content: "# local\n export WORKER_COUNT = '6' # count\nEMAIL_FROM=\"jobs@example.com\"\n", wantCount: 6},
		{name: "missing equals", content: "test-secret\n", wantErr: "line 1"},
		{name: "invalid variable name", content: "BAD-KEY=test-secret\n", wantErr: "line 1"},
		{name: "unclosed quote", content: "RESEND_API_KEY='test-secret\n", wantErr: "line 1"},
		{name: "trailing text", content: "RESEND_API_KEY='test-secret' invalid\n", wantErr: "line 1"},
		{name: "invalid file setting", content: "WORKER_COUNT=abc\n", wantErr: "WORKER_COUNT"},
		{name: "resend missing credentials", content: "EMAIL_PROVIDER=resend\n", wantErr: "RESEND_API_KEY"},
		{name: "resend complete", content: "EMAIL_PROVIDER=resend\nRESEND_API_KEY=test-secret\nEMAIL_FROM=jobs@example.com\n", wantCount: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnv(t)
			if err := os.WriteFile(".env", []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			for k, v := range tt.process {
				t.Setenv(k, v)
			}
			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %s error, got %v", tt.wantErr, err)
				}
				if strings.Contains(err.Error(), "test-secret") {
					t.Fatal("error exposed secret")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Worker.Count != tt.wantCount {
				t.Fatalf("count: got %d, want %d", cfg.Worker.Count, tt.wantCount)
			}
			if tt.name == "file settings" && cfg.Worker.JobTimeout != 2*time.Minute {
				t.Fatal("file duration not loaded")
			}
			if tt.name == "resend complete" && (cfg.Email.ResendAPIKey != "test-secret" || cfg.Email.From != "jobs@example.com") {
				t.Fatal("email settings not loaded")
			}
			if _, ok := os.LookupEnv("EMAIL_FROM"); ok {
				t.Fatal("loader modified process environment")
			}
			// Repeated loads must re-read the file rather than retain stale values.
			if err := os.WriteFile(".env", []byte("WORKER_COUNT=9\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, exists := tt.process["WORKER_COUNT"]; !exists {
				cfg, err = Load()
				if err != nil || cfg.Worker.Count != 9 {
					t.Fatal("file changes were not loaded")
				}
			}
		})
	}
}

func TestReadDotEnvLiteralValues(t *testing.T) {
	t.Chdir(t.TempDir())
	content := "A='value#literal'\nB=value#literal\nC=value # comment\nD=\"$A $(echo ignored)\"\nEMPTY=\n"
	if err := os.WriteFile(".env", []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	env, err := readDotEnv(".env")
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"A": "value#literal", "B": "value#literal", "C": "value", "D": "$A $(echo ignored)", "EMPTY": ""} {
		if env[key] != want {
			t.Fatalf("unexpected value for %s", key)
		}
	}
}
