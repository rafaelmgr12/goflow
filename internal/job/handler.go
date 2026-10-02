package job

import "context"

type Handler interface {
	Handle(ctx context.Context, job Job) error
}
