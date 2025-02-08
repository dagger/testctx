// Package testctx provides a context-aware wrapper around testing.T and testing.B
// with support for middleware and context propagation. It aims to represent what
// *testing.T might have looked like if context.Context existed at the time it was created.
package testctx

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// Common type aliases for convenience
type (
	// T is a wrapper around *testing.T
	T = W[*testing.T]
	// B is a wrapper around *testing.B
	B = W[*testing.B]
	// TestMiddleware is a middleware function that takes a context and T
	TestMiddleware = Middleware[*testing.T]
	// BenchMiddleware is a middleware function that takes a context and B
	BenchMiddleware = Middleware[*testing.B]
	// TestFunc is a test function that takes a context and T
	TestFunc = RunFunc[*testing.T]
	// BenchFunc is a benchmark function that takes a context and B
	BenchFunc = RunFunc[*testing.B]
)

// Runner is a constraint for types that support subtests/subbenchmarks
type Runner[T testing.TB] interface {
	testing.TB
	Run(string, func(T)) bool
}

// Logger represents something that can receive test log messages
type Logger interface {
	Log(args ...any)
	Logf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
}

// W is a context-aware wrapper for test/benchmark types that supports middleware
// and context propagation
type W[T Runner[T]] struct {
	tb         T
	ctx        context.Context
	middleware []Middleware[T]
	logger     Logger

	// we have to embed testing.TB to become a testing.TB ourselves,
	// since it has a private method
	testing.TB
}

// Ensure W implements testing.TB
var _ testing.TB = (*W[*testing.T])(nil)
var _ testing.TB = (*W[*testing.B])(nil)

// Middleware represents a function that can wrap a test function
type Middleware[T Runner[T]] func(RunFunc[T]) RunFunc[T]

// RunFunc represents a test function that takes a context and a wrapper
type RunFunc[T Runner[T]] func(context.Context, *W[T])

// New creates a context-aware test wrapper. The wrapper provides:
//   - Context propagation through test hierarchies
//   - Middleware support for test instrumentation
//   - Logging interception via WithLogger
//
// The context is automatically canceled when the test completes.
// See Using() for details on middleware behavior.
func New[T Runner[T]](t T, middleware ...Middleware[T]) *W[T] {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &W[T]{
		TB:         t,
		tb:         t,
		ctx:        ctx,
		middleware: middleware,
	}
}

// Using adds middleware to the wrapper. Middleware are executed in a nested pattern:
// the first middleware added becomes the outermost wrapper, the last middleware added becomes
// the innermost wrapper. For example:
//
//	t.Using(first, second, third)
//
// Results in the execution order:
//
//	first {
//	    second {
//	        third {
//	            test
//	        }
//	    }
//	}
//
// This pattern ensures that:
// 1. "before" middleware code executes from outside-in (first -> third)
// 2. The test executes
// 3. "after" middleware code executes from inside-out (third -> first)
//
// This matches the behavior of other middleware systems like net/http handlers
// and allows middleware to properly wrap both the setup and cleanup of resources.
func (w *W[T]) Using(m ...Middleware[T]) *W[T] {
	clone := w.clone()
	clone.middleware = append(clone.middleware[:], m...)
	return clone
}

// Unwrap returns the underlying test/benchmark type
func (w *W[T]) Unwrap() T {
	return w.tb
}

// BaseName returns the name of the test without the full path prefix
func (w *W[T]) BaseName() string {
	name := w.Name()
	if idx := lastSlashIndex(name); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// Context returns the current context
func (w *W[T]) Context() context.Context {
	return w.ctx
}

// WithContext creates a new wrapper with the given context
func (w *W[T]) WithContext(ctx context.Context) *W[T] {
	clone := w.clone()
	clone.ctx = ctx
	return clone
}

// Run runs a subtest with the given name and function. The function will be wrapped
// by any middleware registered via Using() or New(), with middleware executing in
// the order described by Using().
func (w *W[T]) Run(name string, fn RunFunc[T]) bool {
	return w.tb.Run(name, func(t T) {
		newW := w.clone()
		newW.tb = t
		newW.TB = t

		wrapped := w.wrapWithMiddleware(fn)
		wrapped(newW.ctx, newW)
	})
}

// WithLogger returns a new wrapper with the given logger. The logger will receive copies
// of all test log messages (Log, Logf), errors (Error, Errorf), fatal errors
// (Fatal, Fatalf), and skip notifications (Skip, Skipf). This allows test output
// to be captured or redirected while still maintaining the original test behavior.
func (w *W[T]) WithLogger(l Logger) *W[T] {
	clone := w.clone()
	clone.logger = l
	return clone
}

// Error calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Error(args ...any) {
	w.tb.Error(args...)
	if w.logger != nil {
		w.logger.Error(args...)
	}
}

// Errorf calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Errorf(format string, args ...any) {
	w.tb.Errorf(format, args...)
	if w.logger != nil {
		w.logger.Errorf(format, args...)
	}
}

// Fatal calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Fatal(args ...any) {
	if w.logger != nil {
		w.logger.Error(args...)
	}
	w.tb.Fatal(args...)
}

// Fatalf calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Fatalf(format string, args ...any) {
	if w.logger != nil {
		w.logger.Errorf(format, args...)
	}
	w.tb.Fatalf(format, args...)
}

// Log calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Log(args ...any) {
	w.tb.Log(args...)
	if w.logger != nil {
		w.logger.Log(args...)
	}
}

// Logf calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Logf(format string, args ...any) {
	w.tb.Logf(format, args...)
	if w.logger != nil {
		w.logger.Logf(format, args...)
	}
}

// Skip calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Skip(args ...any) {
	if w.logger != nil {
		w.logger.Log(args...)
	}
	w.tb.Skip(args...)
}

// Skipf calls through to the underlying test/benchmark type and logs if a logger is set
func (w *W[T]) Skipf(format string, args ...any) {
	if w.logger != nil {
		w.logger.Logf(format, args...)
	}
	w.tb.Skipf(format, args...)
}

// RunTests runs Test* methods from one or more test containers
func (w *W[T]) RunTests(containers ...any) {
	w.runMethods(containers, "Test")
}

// RunBenchmarks runs Benchmark* methods from one or more benchmark containers
func (w *W[T]) RunBenchmarks(containers ...any) {
	w.runMethods(containers, "Benchmark")
}

// runMethods is the internal implementation that handles both types
func (w *W[T]) runMethods(containers []any, prefix string) {
	wrapped := w.wrapWithMiddleware(func(ctx context.Context, t *W[T]) {
		for _, container := range containers {
			containerType := reflect.TypeOf(container)
			containerValue := reflect.ValueOf(container)

			for i := 0; i < containerType.NumMethod(); i++ {
				method := containerType.Method(i)
				if !strings.HasPrefix(method.Name, prefix) {
					continue
				}

				methodType := method.Type
				if methodType.NumIn() != 3 || // receiver + context + W[T]
					!methodType.In(1).AssignableTo(reflect.TypeOf((*context.Context)(nil)).Elem()) ||
					!methodType.In(2).AssignableTo(reflect.TypeOf((*W[T])(nil))) {
					continue
				}

				t.Run(method.Name, func(ctx context.Context, t *W[T]) {
					method.Func.Call([]reflect.Value{
						containerValue,
						reflect.ValueOf(ctx),
						reflect.ValueOf(t),
					})
				})
			}
		}
	})

	wrapped(w.ctx, w)
}

// clone creates a shallow copy of the wrapper with all fields preserved
func (w *W[T]) clone() *W[T] {
	return &W[T]{
		TB:         w.TB,
		tb:         w.tb,
		ctx:        w.ctx,
		middleware: w.middleware,
		logger:     w.logger,
	}
}

func lastSlashIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// wrapWithMiddleware wraps a test function with all registered middleware
func (w *W[T]) wrapWithMiddleware(fn RunFunc[T]) RunFunc[T] {
	// First wrap the function to ensure context sync
	wrapped := func(ctx context.Context, t *W[T]) {
		fn(ctx, t.WithContext(ctx))
	}

	// Walk the middleware in reverse order so the last middleware added
	// becomes the innermost wrapper.
	for i := len(w.middleware) - 1; i >= 0; i-- {
		wrapped = w.middleware[i](wrapped)
	}

	return wrapped
}
