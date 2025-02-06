package otelmw

import (
	"context"
	"testing"

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

// WithTracing creates middleware that adds OpenTelemetry tracing around each test
func WithTracing(cfg ...Config) testctx.Middleware[testing.T] {
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

	return func(next testctx.TestFunc[testing.T]) testctx.TestFunc[testing.T] {
		return func(ctx context.Context, t *testing.T) {
			// Start a new span for this test
			opts := []trace.SpanStartOption{
				trace.WithAttributes(c.Attributes...),
				trace.WithAttributes(
					attribute.String("test.name", t.Name()),
					attribute.String("test.package", t.Name()[:len(t.Name())-len("/"+t.Name())]),
				),
			}

			ctx, span := tracer.Start(ctx, t.Name(), opts...)
			defer func() {
				if t.Failed() {
					span.SetStatus(codes.Error, "test failed")
				}
				span.End()
			}()

			// Run the test with the new context containing the span
			next(ctx, t)
		}
	}
}
