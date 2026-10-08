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

	"github.com/rafaelmgr12/goflow/internal/adapter/email/logsender"
	"github.com/rafaelmgr12/goflow/internal/adapter/email/resend"
	"github.com/rafaelmgr12/goflow/internal/adapter/report/localfile"
	"github.com/rafaelmgr12/goflow/internal/config"
	"github.com/rafaelmgr12/goflow/internal/job"
	"github.com/rafaelmgr12/goflow/internal/processor"
	"github.com/rafaelmgr12/goflow/internal/scheduler"
	"github.com/rafaelmgr12/goflow/internal/storage/postgres"
	"github.com/rafaelmgr12/goflow/internal/tasks/email"
	"github.com/rafaelmgr12/goflow/internal/tasks/report"
	"github.com/rafaelmgr12/goflow/internal/worker"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("loading configuration: ", err)
	}

	var emailSender email.Sender
	switch cfg.Email.Provider {
	case "log":
		emailSender = logsender.New()
	case "resend":
		emailSender = resend.New(cfg.Email.ResendAPIKey, cfg.Email.From)
	default:
		log.Fatal("unsupported EMAIL_PROVIDER: expected log or resend")
	}

	var reportWriter report.Writer
	switch cfg.Report.Provider {
	case "localfile":
		reportWriter = localfile.New(cfg.Report.OutputDir)
	default:
		log.Fatal("unsupported REPORT_PROVIDER: expected localfile")
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

	emailHandler := email.NewHandler(emailSender)
	executor.Register(email.Type, emailHandler)

	reportHandler := report.New(reportWriter)
	executor.Register(report.Type, reportHandler)

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

	// Temporary demo jobs for end-to-end validation.
	now := time.Now().UTC()

	j := job.Job{
		ID:          fmt.Sprintf("job-%d", now.UnixNano()),
		Type:        email.Type,
		Payload:     []byte(`{"to":"rafa@example.com","subject":"GoFlow test","body":"Hello from GoFlow"}`),
		Status:      job.StatusPending,
		ScheduledAt: now,
		CreatedAt:   now,
	}

	if err := repo.Save(ctx, j); err != nil {
		log.Fatal("saving demo job: ", err)
	}

	log.Printf("demo job %s created", j.ID)

	reportJob := job.Job{
		ID:          fmt.Sprintf("report-%d", now.UnixNano()),
		Type:        report.Type,
		Payload:     []byte(`{"title":"Relatório GoFlow","content":"Este relatório foi gerado pelo pipeline de jobs do GoFlow."}`),
		Status:      job.StatusPending,
		ScheduledAt: now,
		CreatedAt:   now,
	}

	if err := repo.Save(ctx, reportJob); err != nil {
		log.Fatal("saving report demo job: ", err)
	}

	log.Printf("report demo job %s created", reportJob.ID)

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
