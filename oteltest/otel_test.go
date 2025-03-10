package oteltest_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dagger/testctx"
	"github.com/dagger/testctx/oteltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestMain(m *testing.M) {
	os.Exit(oteltest.Main(m))
}

func TestOTel(t *testing.T) {
	testctx.New(t,
		testctx.WithParallel(),
		oteltest.WithTracing[*testing.T](),
		oteltest.WithLogging[*testing.T](),
	).RunTests(OTelSuite{})
}

type OTelSuite struct{}

func (OTelSuite) TestParallelAttribution(ctx context.Context, t *testctx.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Run("test", func(ctx context.Context, t *testctx.T) {
		time.Sleep(time.Second)

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			time.Sleep(time.Second)

			t.Run("grandchild", func(ctx context.Context, t *testctx.T) {
				time.Sleep(time.Second)
			})
		})
	})

	t.Run("test 2", func(ctx context.Context, t *testctx.T) {
		time.Sleep(time.Second)

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			time.Sleep(time.Second)

			t.Run("grandchild", func(ctx context.Context, t *testctx.T) {
				time.Sleep(time.Second)
			})
		})
	})

	t.Run("test 3", func(ctx context.Context, t *testctx.T) {
		time.Sleep(time.Second)

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			time.Sleep(time.Second)

			t.Run("grandchild", func(ctx context.Context, t *testctx.T) {
				time.Sleep(time.Second)
			})
		})
	})

	t.Run("test 4", func(ctx context.Context, t *testctx.T) {
		time.Sleep(time.Second)

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			time.Sleep(time.Second)

			t.Run("grandchild", func(ctx context.Context, t *testctx.T) {
				time.Sleep(time.Second)
			})
		})
	})
}

func (OTelSuite) TestAttributes(ctx context.Context, t *testctx.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	tt := testctx.New(t.Unwrap(), oteltest.WithTracing(oteltest.TraceConfig[*testing.T]{
		TracerProvider: tracerProvider,
		Attributes: []attribute.KeyValue{
			attribute.String("test.suite", "otel_test"),
		},
	}))

	tt.Run("passing-test", func(ctx context.Context, t *testctx.T) {
		time.Sleep(time.Second)
	})

	// Verify spans were recorded correctly
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	// Check passing test span
	passSpan := spans[0]
	assert.Equal(t, "passing-test", passSpan.Name())
	assert.Equal(t, codes.Ok, passSpan.Status().Code)
	assert.Contains(t, passSpan.Attributes(), attribute.String("test.suite", "otel_test"))
}

func (OTelSuite) TestStartOptions(ctx context.Context, t *testctx.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	tt := testctx.New(t.Unwrap(), oteltest.WithTracing(oteltest.TraceConfig[*testing.T]{
		TracerProvider: tracerProvider,
		StartOptions: func(w *testctx.W[*testing.T]) []trace.SpanStartOption {
			return []trace.SpanStartOption{
				trace.WithAttributes(attribute.String("test.suite", "otel_test")),
			}
		},
	}))

	tt.Run("passing-test", func(ctx context.Context, t *testctx.T) {
		time.Sleep(time.Second)
	})

	// Verify spans were recorded correctly
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	// Check passing test span
	passSpan := spans[0]
	assert.Equal(t, "passing-test", passSpan.Name())
	assert.Equal(t, codes.Ok, passSpan.Status().Code)
	assert.Contains(t, passSpan.Attributes(), attribute.String("test.suite", "otel_test"))
}

func BenchmarkWithTracing(b *testing.B) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	bb := testctx.New(b, oteltest.WithTracing[*testing.B](oteltest.TraceConfig[*testing.B]{
		TracerProvider: tracerProvider,
	}))

	bb.Run("traced-benchmark", func(ctx context.Context, b *testctx.B) {
		bench := b.Unwrap()
		for i := 0; i < bench.N; i++ {
			time.Sleep(1 * time.Microsecond)
		}
	})

	b.Logf("b.N: %d", b.N)

	// Verify benchmark span was recorded
	spans := spanRecorder.Ended()
	for _, span := range spans {
		// dump all span data
		b.Logf("span: %+v", span)
	}
	require.Len(b, spans, b.N)

	benchSpan := spans[0]
	assert.Equal(b, "traced-benchmark", benchSpan.Name())
	assert.Equal(b, codes.Ok, benchSpan.Status().Code)
}

func (OTelSuite) TestTracingNesting(ctx context.Context, t *testctx.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	tt := testctx.New(t.Unwrap(), oteltest.WithTracing(oteltest.TraceConfig[*testing.T]{
		TracerProvider: tracerProvider,
	}))

	tt.Run("parent", func(ctx context.Context, t *testctx.T) {
		time.Sleep(10 * time.Millisecond)

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			time.Sleep(10 * time.Millisecond)

			t.Run("grandchild", func(ctx context.Context, t *testctx.T) {
				time.Sleep(10 * time.Millisecond)
			})
		})
	})

	spans := spanRecorder.Ended()
	require.Len(t, spans, 3)

	// Spans should end in reverse order (grandchild, child, parent)
	grandchild := spans[0]
	child := spans[1]
	parent := spans[2]

	// Verify names
	assert.Equal(t, "grandchild", grandchild.Name())
	assert.Equal(t, "child", child.Name())
	assert.Equal(t, "parent", parent.Name())

	// Verify span nesting
	assert.Equal(t, child.SpanContext().SpanID(), grandchild.Parent().SpanID())
	assert.Equal(t, parent.SpanContext().SpanID(), child.Parent().SpanID())

	// Verify timing - each span should end after its children
	assert.True(t, grandchild.EndTime().Before(child.EndTime()))
	assert.True(t, child.EndTime().Before(parent.EndTime()))
}

func (OTelSuite) TestLogging(ctx context.Context, t *testctx.T) {
	// pretty annoying, not sure how to test this, just comment out to verify
	t.Skip("skipping logging test since it intentionally fails")

	// Regular logs
	t.Log("simple log message")
	t.Logf("formatted %s message", "log")

	// Error logs
	t.Error("simple error message")
	t.Errorf("formatted %s message", "error")

	// Nested test with logs
	t.Run("child", func(ctx context.Context, t *testctx.T) {
		t.Log("child log message")
		t.Error("child error message")
	})
}

func (OTelSuite) TestInterrupted(ctx context.Context, t *testctx.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	tt := testctx.New(t.Unwrap(),
		testctx.WithTimeout[*testing.T](100*time.Millisecond), // Short timeout to force interruption
		oteltest.WithTracing(oteltest.TraceConfig[*testing.T]{
			TracerProvider: tracerProvider,
		}),
	)

	// Run a test that will time out
	tt.Run("timing-out-test", func(ctx context.Context, t *testctx.T) {
		select {
		case <-ctx.Done():
			// Test should be interrupted by timeout
			return
		case <-time.After(1 * time.Second):
			t.Fatal("test should have timed out")
		}
	})

	// Verify spans were recorded correctly
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	// Check the interrupted test span
	timeoutSpan := spans[0]
	assert.Equal(t, "timing-out-test", timeoutSpan.Name())
	assert.Equal(t, codes.Error, timeoutSpan.Status().Code)
	assert.Equal(t, "test interrupted: context deadline exceeded", timeoutSpan.Status().Description)
}
