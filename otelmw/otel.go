package otelmw

import (
	"context"
	"os"
	"testing"

	"dagger.io/dagger/telemetry"
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

var propagatedCtx = context.Background()

// Main is a helper function that initializes OTel and runs the tests
// before exiting. Use it in your TestMain function.
//
// Main covers initializing the OTel trace and logger providers, pointing
// to standard OTEL_* env vars.
//
// It also initializes a context that will be used to propagate trace
// context to subtests.
func Main(m *testing.M) {
	propagatedCtx = telemetry.InitEmbedded(context.Background(), nil)
	exitCode := m.Run()
	telemetry.Close()
	os.Exit(exitCode)
}

// testSpanKey is the key used to store the test span in the context
type testSpanKey struct{}

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

			spanName := w.BaseName()

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer func() {
				if w.Failed() {
					span.SetStatus(codes.Error, "test failed")
				} else {
					span.SetStatus(codes.Ok, "test passed")
				}
				span.End()
			}()

			// Store the span in the context so that it can be linked to in subtests
			ctx = context.WithValue(ctx, testSpanKey{}, span)

			next(ctx, w)
		}
	}
}
