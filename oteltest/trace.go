package oteltest

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dagger/testctx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TraceConfig holds configuration for the OpenTelemetry tracing middleware
type TraceConfig[T testctx.Runner[T]] struct {
	// TracerProvider to use for creating spans. If nil, the global provider will be used.
	TracerProvider trace.TracerProvider
	// Attributes to add to all test spans
	Attributes []attribute.KeyValue
	// StartOptions allows customizing the span start options for each test/benchmark
	StartOptions func(*testctx.W[T]) []trace.SpanStartOption
}

// testSpanKey is the key used to store the test span in the context
type testSpanKey struct{}

// WithTracing creates middleware that adds OpenTelemetry tracing around each test/benchmark
func WithTracing[T testctx.Runner[T]](cfg ...TraceConfig[T]) testctx.Middleware[T] {
	var c TraceConfig[T]
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.TracerProvider == nil {
		c.TracerProvider = otel.GetTracerProvider()
	}

	tracer := c.TracerProvider.Tracer(
		instrumentationLibrary,
		trace.WithInstrumentationVersion(instrumentationVersion),
	)

	return func(next testctx.RunFunc[T]) testctx.RunFunc[T] {
		return func(ctx context.Context, w *testctx.W[T]) {
			// Inherit from any trace context that Main picked up
			if !trace.SpanContextFromContext(ctx).IsValid() {
				ctx = trace.ContextWithSpanContext(ctx, trace.SpanContextFromContext(propagatedCtx))
			}

			// Start a new span for this test/benchmark
			opts := []trace.SpanStartOption{
				trace.WithAttributes(c.Attributes...),
			}

			// Link to the parent test span so that tools can attribute the subtest
			// runtime to the parent test when tests are run in parallel
			if val, ok := ctx.Value(testSpanKey{}).(trace.Span); ok {
				opts = append(opts, trace.WithLinks(trace.Link{
					SpanContext: val.SpanContext(),
				}))
			}

			if c.StartOptions != nil {
				opts = append(opts, c.StartOptions(w)...)
			}

			spanName := w.BaseName()

			// Accumulate Error/Fatal messages so the span status carries the
			// actual failure reason instead of a generic "test failed".
			errors := &errorAccumulator{}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer func() {
				if ctx.Err() != nil {
					// Test was interrupted (timeout or cancellation)
					span.SetStatus(codes.Error, "test interrupted: "+ctx.Err().Error())
				} else if w.Failed() {
					desc := errors.String()
					if desc == "" {
						desc = "test failed"
					}
					span.SetStatus(codes.Error, desc)
				} else {
					span.SetStatus(codes.Ok, "test passed")
				}
				span.End()
			}()

			// Store the span in the context so that it can be linked to in subtests
			ctx = context.WithValue(ctx, testSpanKey{}, span)

			next(ctx, w.WithLogger(errors))
		}
	}
}

// errorAccumulator is a Logger that captures Error/Errorf messages so they
// can be attached to a span status when the test fails.
type errorAccumulator struct {
	mu       sync.Mutex
	messages []string
}

var _ testctx.Logger = (*errorAccumulator)(nil)

func (a *errorAccumulator) Log(args ...any)                 {}
func (a *errorAccumulator) Logf(format string, args ...any) {}

func (a *errorAccumulator) Error(args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, fmt.Sprint(args...))
}

func (a *errorAccumulator) Errorf(format string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, fmt.Sprintf(format, args...))
}

// String returns all accumulated error messages joined by newlines.
func (a *errorAccumulator) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.Join(a.messages, "\n")
}
