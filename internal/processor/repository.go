package processor

import "context"

type Repository interface {
	MarkRunning(ctx context.Context, id string) error
	MarkCompleted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string) error
}
