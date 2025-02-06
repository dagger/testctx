package otelmw

import (
	"context"

	"github.com/dagger/testctx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Config holds configuration for the OpenTelemetry middleware
type Config struct {
	// TracerProvider to use for creating spans. If nil, the global provider will be used.
	TracerProvider trace.TracerProvider
	// Attributes to add to all test spans
	Attributes []attribute.KeyValue
}

// WithTracing creates middleware that adds OpenTelemetry tracing around each test/benchmark
func WithTracing[T testctx.Runner[T]](cfg ...Config) testctx.Middleware[T] {
	var c Config
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.TracerProvider == nil {
		c.TracerProvider = otel.GetTracerProvider()
	}

	tracer := c.TracerProvider.Tracer(
		"github.com/dagger/testctx/otelmw",
		trace.WithInstrumentationVersion("v0.1.0"),
	)

	return func(next testctx.TestFunc[T]) testctx.TestFunc[T] {
		return func(ctx context.Context, w *testctx.W[T]) {
			testName := w.Name()

			// Start a new span for this test/benchmark
			opts := []trace.SpanStartOption{
				trace.WithAttributes(c.Attributes...),
			}

			ctx, span := tracer.Start(ctx, testName, opts...)
			defer func() {
				if w.Failed() {
					span.SetStatus(codes.Error, "test failed")
				} else {
					span.SetStatus(codes.Ok, "test passed")
				}
				span.End()
			}()

			next(ctx, w)
		}
	}
}
