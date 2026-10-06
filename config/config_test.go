package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

var envNames = []string{
	"DATABASE_URL", "WORKER_COUNT", "WORKER_JOB_TIMEOUT",
	"SCHEDULER_BATCH_SIZE", "SCHEDULER_POLL_INTERVAL",
	"EMAIL_PROVIDER", "RESEND_API_KEY", "EMAIL_FROM",
}

func cleanEnv(t *testing.T) {
	t.Helper()
	for _, name := range envNames {
		// Setenv registers restoration and prevents parallel execution.
		t.Setenv(name, os.Getenv(name))
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoad(t *testing.T) {
	defaults := Config{
		Database:  DatabaseConfig{URL: "postgres://goflow:goflow@localhost:5432/goflow?sslmode=disable"},
		Worker:    WorkerConfig{Count: 3, JobTimeout: 30 * time.Second},
		Scheduler: SchedulerConfig{BatchSize: 100, PollInterval: 5 * time.Second},
		Email:     EmailConfig{Provider: "log"},
	}
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{name: "defaults", want: defaults},
		{name: "explicit log without credentials", env: map[string]string{"EMAIL_PROVIDER": "log"}, want: defaults},
		{name: "custom values", env: map[string]string{
			"DATABASE_URL": "postgres://localhost/custom", "WORKER_COUNT": "7", "WORKER_JOB_TIMEOUT": "1m30s",
			"SCHEDULER_BATCH_SIZE": "25", "SCHEDULER_POLL_INTERVAL": "250ms",
			"EMAIL_PROVIDER": "resend", "RESEND_API_KEY": "test-secret", "EMAIL_FROM": "jobs@example.com",
		}, want: Config{
			Database:  DatabaseConfig{URL: "postgres://localhost/custom"},
			Worker:    WorkerConfig{Count: 7, JobTimeout: 90 * time.Second},
			Scheduler: SchedulerConfig{BatchSize: 25, PollInterval: 250 * time.Millisecond},
			Email:     EmailConfig{Provider: "resend", ResendAPIKey: "test-secret", From: "jobs@example.com"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnv(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}
			got, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatal("configuration differs from expected settings")
			}
		})
	}
}

func TestLoadInvalidSettings(t *testing.T) {
	tests := []struct{ name, value, message string }{
		{"DATABASE_URL", "", "must not be empty"},
		{"EMAIL_PROVIDER", "", "must not be empty"},
	}
	for _, name := range []string{"WORKER_COUNT", "SCHEDULER_BATCH_SIZE"} {
		for _, value := range []string{"", "abc", "1.5", "999999999999999999999999999999"} {
			tests = append(tests, struct{ name, value, message string }{name, value, "valid integer"})
		}
		for _, value := range []string{"0", "-1"} {
			tests = append(tests, struct{ name, value, message string }{name, value, "greater than zero"})
		}
	}
	for _, name := range []string{"WORKER_JOB_TIMEOUT", "SCHEDULER_POLL_INTERVAL"} {
		for _, value := range []string{"", "abc", "30", "999999999999999999999h"} {
			tests = append(tests, struct{ name, value, message string }{name, value, "valid duration"})
		}
		for _, value := range []string{"0s", "-1s"} {
			tests = append(tests, struct{ name, value, message string }{name, value, "greater than zero"})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.value, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv(tt.name, tt.value)
			got, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.name) || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("expected error identifying %s and %s, got %v", tt.name, tt.message, err)
			}
			if got != (Config{}) {
				t.Fatal("expected zero configuration on failure")
			}
		})
	}
}

func TestLoadEmailRequirements(t *testing.T) {
	tests := []struct{ name, provider, key, from, missing string }{
		{name: "log without credentials", provider: "log"},
		{name: "future provider without Resend credentials", provider: "ses"},
		{name: "resend missing both", provider: "resend", missing: "RESEND_API_KEY"},
		{name: "resend missing key", provider: "resend", from: "jobs@example.com", missing: "RESEND_API_KEY"},
		{name: "resend missing from", provider: "resend", key: "test-secret", missing: "EMAIL_FROM"},
		{name: "resend complete", provider: "resend", key: "test-secret", from: "jobs@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("EMAIL_PROVIDER", tt.provider)
			if tt.key != "" {
				t.Setenv("RESEND_API_KEY", tt.key)
			}
			if tt.from != "" {
				t.Setenv("EMAIL_FROM", tt.from)
			}
			_, err := Load()
			if tt.missing == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.missing) {
				t.Fatalf("expected required variable %s, got %v", tt.missing, err)
			}
			if err != nil && strings.Contains(err.Error(), "test-secret") {
				t.Fatal("error exposed secret")
			}
		})
	}
}
