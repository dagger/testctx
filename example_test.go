package testctx_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagger/testctx"
)

func TestExample(t *testing.T) {
	tt := testctx.New(t)

	// Add middleware
	tt.Use(
		testctx.WithTimeout(5*time.Second),
		testctx.WithParallel(),
		testctx.WithInstrumentation(func(ctx context.Context, name string) {
			// Add your instrumentation here
		}),
	)

	tt.Run("subtest", func(ctx context.Context, t *testctx.T) {
		// Use context-aware test
		select {
		case <-ctx.Done():
			t.Fatal("test timed out")
		case <-time.After(1 * time.Second):
			// Test passed
		}
	})
}
