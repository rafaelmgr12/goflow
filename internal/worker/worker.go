package worker

import (
	"context"
	"log"
	"time"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Processor interface {
	Process(ctx context.Context, job job.Job) error
}

func Run(
	ctx context.Context,
	id int,
	jobs <-chan job.Job,
	processor Processor,
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

			func() {
				baseCtx := context.WithoutCancel(ctx)

				jobCtx, cancel := context.WithTimeout(
					baseCtx,
					30*time.Second,
				)
				defer cancel()

				if err := processor.Process(jobCtx, j); err != nil {
					log.Printf(
						"worker %d failed job %s: %v",
						id,
						j.ID,
						err,
					)
				}
			}()
		}
	}
}
