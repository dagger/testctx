package testctx

import (
	"context"
	"reflect"
	"strings"
	"testing"
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

// W is a context-aware wrapper for test/benchmark types
type W[T Runner[T]] struct {
	tb         T
	ctx        context.Context
	middleware []Middleware[T]
	logger     Logger
}

// Middleware represents a function that can wrap a test function
type Middleware[T Runner[T]] func(RunFunc[T]) RunFunc[T]

// RunFunc represents a test function that takes a context and a wrapper
type RunFunc[T Runner[T]] func(context.Context, *W[T])

// New creates a new context-aware test helper
func New[T Runner[T]](t T, middleware ...Middleware[T]) *W[T] {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &W[T]{
		tb:         t,
		ctx:        ctx,
		middleware: middleware,
	}
}

// Name returns the name of the test
func (w *W[T]) Name() string { return w.tb.Name() }

// BaseName returns the name of the test without the full path prefix
func (w *W[T]) BaseName() string {
	name := w.Name()
	if idx := lastSlashIndex(name); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// WithContext creates a new wrapper with the given context
func (w *W[T]) WithContext(ctx context.Context) *W[T] {
	return &W[T]{
		tb:         w.tb,
		ctx:        ctx,
		middleware: w.middleware,
		logger:     w.logger,
	}
}

// Using returns a new wrapper with the given middleware
func (w *W[T]) Using(m ...Middleware[T]) *W[T] {
	return &W[T]{
		tb:         w.tb,
		ctx:        w.ctx,
		middleware: append(w.middleware[:], m...),
		logger:     w.logger,
	}
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
			logger:     w.logger,
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

// WithLogger returns a new wrapper with the given logger
func (w *W[T]) WithLogger(l Logger) *W[T] {
	return &W[T]{
		tb:         w.tb,
		ctx:        w.ctx,
		middleware: w.middleware,
		logger:     l,
	}
}

func (w *W[T]) Error(args ...any) {
	w.tb.Error(args...)
	if w.logger != nil {
		w.logger.Error(args...)
	}
}

func (w *W[T]) Errorf(format string, args ...any) {
	w.tb.Errorf(format, args...)
	if w.logger != nil {
		w.logger.Errorf(format, args...)
	}
}

func (w *W[T]) Fail()        { w.tb.Fail() }
func (w *W[T]) FailNow()     { w.tb.FailNow() }
func (w *W[T]) Failed() bool { return w.tb.Failed() }
func (w *W[T]) Fatal(args ...any) {
	if w.logger != nil {
		w.logger.Error(args...)
	}
	w.tb.Fatal(args...)
}

func (w *W[T]) Fatalf(format string, args ...any) {
	if w.logger != nil {
		w.logger.Errorf(format, args...)
	}
	w.tb.Fatalf(format, args...)
}

func (w *W[T]) Helper() { w.tb.Helper() }
func (w *W[T]) Log(args ...any) {
	w.tb.Log(args...)
	if w.logger != nil {
		w.logger.Log(args...)
	}
}

func (w *W[T]) Logf(format string, args ...any) {
	w.tb.Logf(format, args...)
	if w.logger != nil {
		w.logger.Logf(format, args...)
	}
}

func (w *W[T]) Skip(args ...any) {
	if w.logger != nil {
		w.logger.Log(args...)
	}
	w.tb.Skip(args...)
}

func (w *W[T]) SkipNow() { w.tb.SkipNow() }
func (w *W[T]) Skipf(format string, args ...any) {
	if w.logger != nil {
		w.logger.Logf(format, args...)
	}
	w.tb.Skipf(format, args...)
}

func (w *W[T]) Skipped() bool   { return w.tb.Skipped() }
func (w *W[T]) TempDir() string { return w.tb.TempDir() }

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

func lastSlashIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// RunSuite runs all test methods on the given suite as subtests
func (w *W[T]) RunSuite(s any) {
	suiteType := reflect.TypeOf(s)
	suiteValue := reflect.ValueOf(s)

	for i := 0; i < suiteType.NumMethod(); i++ {
		method := suiteType.Method(i)
		if !strings.HasPrefix(method.Name, "Test") {
			continue
		}

		methodType := method.Type
		if methodType.NumIn() != 3 || // receiver + context + W[T]
			!methodType.In(1).AssignableTo(reflect.TypeOf((*context.Context)(nil)).Elem()) ||
			!methodType.In(2).AssignableTo(reflect.TypeOf((*W[T])(nil))) {
			continue
		}

		// Run each test method as a subtest
		w.Run(method.Name, func(ctx context.Context, t *W[T]) {
			method.Func.Call([]reflect.Value{
				suiteValue,
				reflect.ValueOf(ctx),
				reflect.ValueOf(t),
			})
		})
	}
}
