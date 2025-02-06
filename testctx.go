package testctx

import (
	"context"
	"sync"
	"testing"
)

// T is a context-aware test helper that wraps *testing.T
type T struct {
	*testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	middleware []Middleware
	mu         sync.RWMutex
	cleanup    []func()
}

// Middleware represents a function that can wrap a test function
type Middleware func(TestFunc) TestFunc

// TestFunc represents a test function that takes a context and a T
type TestFunc func(context.Context, *T)

// New creates a new context-aware test helper
func New(t *testing.T) *T {
	ctx, cancel := context.WithCancel(context.Background())
	return &T{
		T:      t,
		ctx:    ctx,
		cancel: cancel,
	}
}

// WithContext creates a new T with the given context
func (t *T) WithContext(ctx context.Context) *T {
	ctx, cancel := context.WithCancel(ctx)
	return &T{
		T:          t.T,
		ctx:        ctx,
		cancel:     cancel,
		middleware: t.middleware,
	}
}

// Use adds middleware to the test helper
func (t *T) Use(m ...Middleware) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.middleware = append(t.middleware, m...)
}

// Context returns the current context
func (t *T) Context() context.Context {
	return t.ctx
}

// Run runs a subtest with the given name and function
func (t *T) Run(name string, fn TestFunc) bool {
	return t.T.Run(name, func(tt *testing.T) {
		newT := &T{
			T:          tt,
			ctx:        t.ctx,
			middleware: t.middleware,
		}

		wrapped := fn
		// Apply middleware in reverse order
		for i := len(t.middleware) - 1; i >= 0; i-- {
			wrapped = t.middleware[i](wrapped)
		}

		wrapped(newT.ctx, newT)
	})
}

// Cleanup registers a function to be called when the test completes
func (t *T) Cleanup(f func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanup = append(t.cleanup, f)
	t.T.Cleanup(f)
}
