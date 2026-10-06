// Package config loads and validates application settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database  DatabaseConfig
	Scheduler SchedulerConfig
	Worker    WorkerConfig
	Email     EmailConfig
}

type DatabaseConfig struct{ URL string }
type SchedulerConfig struct {
	BatchSize    int
	PollInterval time.Duration
}
type WorkerConfig struct {
	Count      int
	JobTimeout time.Duration
}
type EmailConfig struct {
	Provider     string
	ResendAPIKey string
	From         string
}

// Load uses local defaults for unset variables. Explicitly empty settings are
// invalid, except for email credentials when the provider is not resend.
// Provider support and adapter construction belong to the application.
func Load() (Config, error) {
	var cfg Config
	cfg.Database.URL = envOrDefault("DATABASE_URL", "postgres://goflow:goflow@localhost:5432/goflow?sslmode=disable")
	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must not be empty")
	}
	var err error
	cfg.Worker.Count, err = positiveInt("WORKER_COUNT", "3")
	if err != nil {
		return Config{}, err
	}
	cfg.Worker.JobTimeout, err = positiveDuration("WORKER_JOB_TIMEOUT", "30s")
	if err != nil {
		return Config{}, err
	}
	cfg.Scheduler.BatchSize, err = positiveInt("SCHEDULER_BATCH_SIZE", "100")
	if err != nil {
		return Config{}, err
	}
	cfg.Scheduler.PollInterval, err = positiveDuration("SCHEDULER_POLL_INTERVAL", "5s")
	if err != nil {
		return Config{}, err
	}
	cfg.Email = EmailConfig{
		Provider:     envOrDefault("EMAIL_PROVIDER", "log"),
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		From:         os.Getenv("EMAIL_FROM"),
	}
	if cfg.Email.Provider == "" {
		return Config{}, fmt.Errorf("EMAIL_PROVIDER must not be empty")
	}
	if cfg.Email.Provider == "resend" {
		if cfg.Email.ResendAPIKey == "" {
			return Config{}, fmt.Errorf("RESEND_API_KEY is required when EMAIL_PROVIDER=resend")
		}
		if cfg.Email.From == "" {
			return Config{}, fmt.Errorf("EMAIL_FROM is required when EMAIL_PROVIDER=resend")
		}
	}
	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func positiveInt(name, fallback string) (int, error) {
	value, err := strconv.Atoi(envOrDefault(name, fallback))
	// Do not include environment values in errors: they may contain secrets.
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", name)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return value, nil
}

func positiveDuration(name, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(envOrDefault(name, fallback))
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration (for example, 30s)", name)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return value, nil
}
