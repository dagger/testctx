package otelmw_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagger/testctx"
	"github.com/dagger/testctx/otelmw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWithTracing(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	tt := testctx.New(t)
	tt.Use(otelmw.WithTracing[*testing.T](otelmw.Config{
		TracerProvider: tracerProvider,
		Attributes: []attribute.KeyValue{
			attribute.String("test.suite", "otel_test"),
		},
	}))

	tt.Run("passing-test", func(ctx context.Context, t *testctx.T) {
		time.Sleep(100 * time.Millisecond)
	})

	tt.Run("failing-test", func(ctx context.Context, t *testctx.T) {
		t.Error("something went wrong")
	})

	// Verify spans were recorded correctly
	spans := spanRecorder.Ended()
	require.Len(t, spans, 2)

	// Check passing test span
	passSpan := spans[0]
	assert.Equal(t, "TestWithTracing/passing-test", passSpan.Name())
	assert.Equal(t, codes.Ok, passSpan.Status().Code)
	assert.Contains(t, passSpan.Attributes(), attribute.String("test.suite", "otel_test"))

	// Check failing test span
	failSpan := spans[1]
	assert.Equal(t, "TestWithTracing/failing-test", failSpan.Name())
	assert.Equal(t, codes.Error, failSpan.Status().Code)
	assert.Equal(t, "test failed", failSpan.Status().Description)
}

func BenchmarkWithTracing(b *testing.B) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	bb := testctx.New(b)
	bb.Use(otelmw.WithTracing[*testing.B](otelmw.Config{
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
	assert.Equal(b, "BenchmarkWithTracing/traced-benchmark", benchSpan.Name())
	assert.Equal(b, codes.Ok, benchSpan.Status().Code)
}

func TestTracingNesting(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	tt := testctx.New(t)
	tt.Use(otelmw.WithTracing[*testing.T](otelmw.Config{
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
	assert.Equal(t, "TestTracingNesting/parent/child/grandchild", grandchild.Name())
	assert.Equal(t, "TestTracingNesting/parent/child", child.Name())
	assert.Equal(t, "TestTracingNesting/parent", parent.Name())

	// Verify span nesting
	assert.Equal(t, child.SpanContext().SpanID(), grandchild.Parent().SpanID())
	assert.Equal(t, parent.SpanContext().SpanID(), child.Parent().SpanID())

	// Verify timing - each span should end after its children
	assert.True(t, grandchild.EndTime().Before(child.EndTime()))
	assert.True(t, child.EndTime().Before(parent.EndTime()))
}
