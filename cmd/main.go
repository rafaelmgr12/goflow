package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rafaelmgr12/goflow/config"
	"github.com/rafaelmgr12/goflow/internal/job"
	"github.com/rafaelmgr12/goflow/internal/processor"
	"github.com/rafaelmgr12/goflow/internal/scheduler"
	"github.com/rafaelmgr12/goflow/internal/storage/postgres"
	"github.com/rafaelmgr12/goflow/internal/worker"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("loading configuration: ", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	db, err := sql.Open(
		"pgx",
		cfg.Database.URL,
	)
	if err != nil {
		log.Fatal("opening postgres: ", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatal("postgres unavailable: ", err)
	}

	log.Println("connected to postgres")

	repo := postgres.NewJobRepository(db)

	executor := job.NewExecutor()

	executor.Register(
		"send_email",
		job.EmailHandler{},
	)

	jobProcessor := processor.NewProcessor(
		repo,
		executor,
	)

	jobs := make(chan job.Job)

	var workers sync.WaitGroup

	for id := 1; id <= cfg.Worker.Count; id++ {
		workers.Add(1)

		go func(workerID int) {
			defer workers.Done()

			worker.Run(
				ctx,
				workerID,
				jobs,
				jobProcessor,
				cfg.Worker.JobTimeout,
			)
		}(id)
	}

	schedulerService, err := scheduler.NewScheduler(
		repo,
		jobs,
		cfg.Scheduler.BatchSize,
		cfg.Scheduler.PollInterval,
	)
	if err != nil {
		log.Fatal("creating scheduler: ", err)
	}

	schedulerDone := make(chan error, 1)

	go func() {
		schedulerDone <- schedulerService.Run(ctx)
	}()

	// Temporary demo job for end-to-end validation.
	now := time.Now().UTC()

	j := job.Job{
		ID:          fmt.Sprintf("job-%d", now.UnixNano()),
		Type:        "send_email",
		Payload:     []byte(`{"to":"rafa@example.com"}`),
		Status:      job.StatusPending,
		ScheduledAt: now,
		CreatedAt:   now,
	}

	if err := repo.Save(ctx, j); err != nil {
		log.Fatal("saving demo job: ", err)
	}

	log.Printf("demo job %s created", j.ID)

	<-ctx.Done()

	log.Println("shutdown requested")

	workers.Wait()

	schedulerErr := <-schedulerDone
	if schedulerErr != nil &&
		!errors.Is(schedulerErr, context.Canceled) {
		log.Printf(
			"scheduler stopped with error: %v",
			schedulerErr,
		)
	}

	log.Println("shutdown complete")
}
