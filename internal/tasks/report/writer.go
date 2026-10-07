package report

import "context"

type Writer interface {
	Write(ctx context.Context, document Document) error
}
