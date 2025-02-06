package testctx

import (
	"context"
	"sync"
	"testing"
)

// Runner is a constraint for types that support subtests/subbenchmarks
type Runner[T testing.TB] interface {
	testing.TB
	Run(string, func(T)) bool
}

// W is a context-aware wrapper for test/benchmark types
type W[T Runner[T]] struct {
	tb         T
	ctx        context.Context
	middleware []Middleware[T]
	mu         sync.RWMutex
}

// Middleware represents a function that can wrap a test function
type Middleware[T Runner[T]] func(RunFunc[T]) RunFunc[T]

// RunFunc represents a test function that takes a context and a wrapper
type RunFunc[T Runner[T]] func(context.Context, *W[T])

// New creates a new context-aware test helper
func New[T Runner[T]](ctx context.Context, t T) *W[T] {
	return &W[T]{
		tb:  t,
		ctx: ctx,
	}
}

// WithContext creates a new wrapper with the given context
func (w *W[T]) WithContext(ctx context.Context) *W[T] {
	return &W[T]{
		tb:         w.tb,
		ctx:        ctx,
		middleware: w.middleware,
	}
}

// Use adds middleware to the test helper
func (w *W[T]) Use(m ...Middleware[T]) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.middleware = append(w.middleware, m...)
}

// Context returns the current context
func (w *W[T]) Context() context.Context {
	return w.ctx
}

// Run runs a subtest with the given name and function
func (w *W[T]) Run(name string, fn RunFunc[T]) bool {
	return w.tb.Run(name, func(t T) {
		newW := &W[T]{
			tb:         t,
			ctx:        w.ctx,
			middleware: w.middleware,
		}

		// First wrap the function to ensure context sync
		wrapped := func(ctx context.Context, t *W[T]) {
			fn(ctx, t.WithContext(ctx))
		}

		// Then apply middleware in reverse order
		for i := len(w.middleware) - 1; i >= 0; i-- {
			wrapped = w.middleware[i](wrapped)
		}

		wrapped(newW.ctx, newW)
	})
}

// Cleanup registers a function to be called when the test completes
func (w *W[T]) Cleanup(f func()) {
	w.tb.Cleanup(f)
}

// Forward testing.TB methods
func (w *W[T]) Error(args ...any)                 { w.tb.Error(args...) }
func (w *W[T]) Errorf(format string, args ...any) { w.tb.Errorf(format, args...) }
func (w *W[T]) Fail()                             { w.tb.Fail() }
func (w *W[T]) FailNow()                          { w.tb.FailNow() }
func (w *W[T]) Failed() bool                      { return w.tb.Failed() }
func (w *W[T]) Fatal(args ...any)                 { w.tb.Fatal(args...) }
func (w *W[T]) Fatalf(format string, args ...any) { w.tb.Fatalf(format, args...) }
func (w *W[T]) Helper()                           { w.tb.Helper() }
func (w *W[T]) Log(args ...any)                   { w.tb.Log(args...) }
func (w *W[T]) Logf(format string, args ...any)   { w.tb.Logf(format, args...) }
func (w *W[T]) Name() string                      { return w.tb.Name() }
func (w *W[T]) Skip(args ...any)                  { w.tb.Skip(args...) }
func (w *W[T]) SkipNow()                          { w.tb.SkipNow() }
func (w *W[T]) Skipf(format string, args ...any)  { w.tb.Skipf(format, args...) }
func (w *W[T]) Skipped() bool                     { return w.tb.Skipped() }
func (w *W[T]) TempDir() string                   { return w.tb.TempDir() }

// Unwrap returns the underlying test/benchmark type
func (w *W[T]) Unwrap() T {
	return w.tb
}

// Common type aliases for convenience
type (
	// T is a wrapper around *testing.T
	T = W[*testing.T]
	// B is a wrapper around *testing.B
	B = W[*testing.B]
	// TestFunc is a test function that takes a context and T
	TestFunc = RunFunc[*testing.T]
	// BenchFunc is a benchmark function that takes a context and B
	BenchFunc = RunFunc[*testing.B]
)

// BaseName returns the name of the test without the full path prefix
func (w *W[T]) BaseName() string {
	name := w.Name()
	if idx := lastSlashIndex(name); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func lastSlashIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
