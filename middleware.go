package testctx

import (
	"context"
	"time"
)

// WithTimeout creates middleware that adds a timeout to the test context
func WithTimeout(d time.Duration) Middleware {
	return func(next TestFunc) TestFunc {
		return func(ctx context.Context, t *T) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			next(ctx, t)
		}
	}
}

// WithParallel creates middleware that runs tests in parallel
func WithParallel() Middleware {
	return func(next TestFunc) TestFunc {
		return func(ctx context.Context, t *T) {
			t.Parallel()
			next(ctx, t)
		}
	}
}

// WithInstrumentation creates middleware that adds instrumentation
func WithInstrumentation(instrument func(context.Context, string)) Middleware {
	return func(next TestFunc) TestFunc {
		return func(ctx context.Context, t *T) {
			instrument(ctx, t.Name())
			next(ctx, t)
		}
	}
}
