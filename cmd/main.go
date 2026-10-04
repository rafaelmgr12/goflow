package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rafaelmgr12/goflow/internal/job"
	"github.com/rafaelmgr12/goflow/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	executor := job.NewExecutor()

	executor.Register(
		"send_email",
		job.EmailHandler{},
	)

	jobs := make(chan job.Job)

	var wg sync.WaitGroup

	for i := 1; i <= 3; i++ {
		wg.Add(1)

		go func(workerID int) {
			defer wg.Done()

			worker.Run(
				ctx,
				workerID,
				jobs,
				executor,
			)
		}(i)
	}

	for i := 1; i <= 100; i++ {
		j := job.Job{
			ID:     fmt.Sprintf("job-%d", i),
			Type:   "send_email",
			Status: job.StatusPending,
		}

		select {
		case <-ctx.Done():
			log.Println("producer shutting down")

			close(jobs)

			wg.Wait()

			log.Println("shutdown complete")
			return

		case jobs <- j:
		}
	}

	close(jobs)

	wg.Wait()

	log.Println("all jobs completed")
}
