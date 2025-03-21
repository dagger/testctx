package testctx

import (
	"context"
	"testing"
	"time"
)

// WithTimeout creates middleware that adds a timeout to the test context
func WithTimeout[T Runner[T]](d time.Duration) Middleware[T] {
	return func(next RunFunc[T]) RunFunc[T] {
		return func(ctx context.Context, t *W[T]) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			next(ctx, t)
		}
	}
}

// WithParallel creates middleware that runs tests in parallel
func WithParallel() Middleware[*testing.T] {
	return func(next TestFunc) TestFunc {
		return func(ctx context.Context, t *W[*testing.T]) {
			t.Unwrap().Parallel()
			next(ctx, t)
		}
	}
}
