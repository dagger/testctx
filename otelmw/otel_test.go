package otelmw_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagger/testctx"
	"github.com/dagger/testctx/otelmw"
	"go.opentelemetry.io/otel/attribute"
)

func TestWithTracing(t *testing.T) {
	tt := testctx.New(t)

	// Add the tracing middleware with custom attributes
	tt.Use(otelmw.WithTracing[*testing.T](otelmw.Config{
		Attributes: []attribute.KeyValue{
			attribute.String("environment", "test"),
		},
	}))

	tt.Run("traced-test", func(ctx context.Context, t *testctx.T) {
		// This test will automatically get a span
		time.Sleep(100 * time.Millisecond)
	})

	tt.Run("failing-test", func(ctx context.Context, t *testctx.T) {
		// This span will be marked with error status
		t.Error("something went wrong")
	})
}
