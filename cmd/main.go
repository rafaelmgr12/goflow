package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
	"github.com/rafaelmgr12/goflow/internal/storage/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	db, err := sql.Open(
		"pgx",
		"postgres://goflow:goflow@localhost:5432/goflow?sslmode=disable",
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
		log.Fatal("saving job: ", err)
	}

	log.Printf("job %s saved successfully", j.ID)

	savedJob, err := repo.FindByID(ctx, j.ID)
	if err != nil {
		log.Fatal("finding job: ", err)
	}

	log.Printf(
		"job loaded: id=%s type=%s status=%s",
		savedJob.ID,
		savedJob.Type,
		savedJob.Status,
	)
}
