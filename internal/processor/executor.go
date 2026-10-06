package processor

import (
	"context"

	"github.com/rafaelmgr12/goflow/internal/job"
)

type Executor interface {
	Execute(ctx context.Context, j job.Job) error
}
