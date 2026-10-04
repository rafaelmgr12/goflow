package worker

import (
	"context"
	"log"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
)

func Run(
	ctx context.Context,
	id int,
	jobs <-chan job.Job,
	executor *job.Executor,
) {
	for {
		select {
		case <-ctx.Done():
			log.Printf(
				"worker %d shutting down",
				id,
			)
			return

		case j, ok := <-jobs:
			if !ok {
				log.Printf(
					"worker %d jobs channel closed",
					id,
				)
				return
			}

			log.Printf(
				"worker %d processing job %s",
				id,
				j.ID,
			)

			baseCtx := context.WithoutCancel(ctx)

			jobCtx, cancel := context.WithTimeout(
				baseCtx,
				30*time.Second,
			)

			if err := executor.Execute(jobCtx, j); err != nil {
				log.Printf(
					"worker %d failed job %s: %v",
					id,
					j.ID,
					err,
				)
			}
			cancel()
		}
	}
}
