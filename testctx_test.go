package testctx_test

import (
	"context"
	"testing"

	"github.com/dagger/testctx"
	"github.com/stretchr/testify/assert"
)

func TestMiddlewareInvocation(t *testing.T) {
	var invocations []string

	tt := testctx.New(t)
	tt.Use(func(next testctx.TestFunc[*testing.T]) testctx.TestFunc[*testing.T] {
		return func(ctx context.Context, t *testctx.W[*testing.T]) {
			invocations = append(invocations, "before:outer")
			next(ctx, t)
			invocations = append(invocations, "after:outer")
		}
	})

	tt.Use(func(next testctx.TestFunc[*testing.T]) testctx.TestFunc[*testing.T] {
		return func(ctx context.Context, t *testctx.W[*testing.T]) {
			invocations = append(invocations, "before:inner")
			next(ctx, t)
			invocations = append(invocations, "after:inner")
		}
	})

	tt.Run("subtest", func(ctx context.Context, t *testctx.T) {
		invocations = append(invocations, "test")
	})

	// Middleware should be applied in order, with inner middleware wrapped by outer
	assert.Equal(t, []string{
		"before:outer",
		"before:inner",
		"test",
		"after:inner",
		"after:outer",
	}, invocations)
}

func TestMiddlewareReuse(t *testing.T) {
	var count int

	tt := testctx.New(t)
	tt.Use(func(next testctx.TestFunc[*testing.T]) testctx.TestFunc[*testing.T] {
		count++
		return func(ctx context.Context, t *testctx.W[*testing.T]) {
			next(ctx, t)
		}
	})

	tt.Run("first", func(ctx context.Context, t *testctx.T) {})
	tt.Run("second", func(ctx context.Context, t *testctx.T) {})

	// Now that each subtest calls the middleware factory, expect count == 2
	assert.Equal(t, 2, count)
}

func TestCleanup(t *testing.T) {
	var cleanupOrder []string

	// Register our assertion first, so it runs last
	t.Cleanup(func() {
		assert.Equal(t, []string{
			"child2",
			"child1",
			"another",
			"parent",
		}, cleanupOrder)
	})

	tt := testctx.New(t)
	tt.Cleanup(func() {
		cleanupOrder = append(cleanupOrder, "parent")
	})

	tt.Run("nested", func(ctx context.Context, t *testctx.T) {
		t.Cleanup(func() {
			cleanupOrder = append(cleanupOrder, "child1")
		})
		t.Cleanup(func() {
			cleanupOrder = append(cleanupOrder, "child2")
		})
	})

	tt.Run("another", func(ctx context.Context, t *testctx.T) {
		t.Cleanup(func() {
			cleanupOrder = append(cleanupOrder, "another")
		})
	})
}

func TestContextPropagation(t *testing.T) {
	type ctxKey struct{}

	tt := testctx.New(t)
	tt.Use(func(next testctx.TestFunc[*testing.T]) testctx.TestFunc[*testing.T] {
		return func(ctx context.Context, t *testctx.W[*testing.T]) {
			count := 0
			if v := ctx.Value(ctxKey{}); v != nil {
				count = v.(int)
			}
			ctx = context.WithValue(ctx, ctxKey{}, count+1)
			next(ctx, t)
		}
	})

	tt.Run("parent", func(ctx context.Context, t *testctx.T) {
		assert.Equal(t, 1, ctx.Value(ctxKey{}))

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			assert.Equal(t, 2, ctx.Value(ctxKey{}))

			t.Run("grandchild", func(ctx context.Context, t *testctx.T) {
				assert.Equal(t, 3, ctx.Value(ctxKey{}))
			})
		})
	})
}

func TestMiddlewareNesting(t *testing.T) {
	var callCount int

	tt := testctx.New(t)
	tt.Use(func(next testctx.TestFunc[*testing.T]) testctx.TestFunc[*testing.T] {
		return func(ctx context.Context, t *testctx.W[*testing.T]) {
			callCount++
			next(ctx, t)
		}
	})

	tt.Run("parent", func(ctx context.Context, t *testctx.T) {
		assert.Equal(t, 1, callCount, "middleware should be called once for parent")

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			assert.Equal(t, 2, callCount, "middleware should be called again for child")

			t.Run("grandchild", func(ctx context.Context, t *testctx.T) {
				assert.Equal(t, 3, callCount, "middleware should be called again for grandchild")
			})
		})
	})
}

func TestMiddlewareDynamicAddition(t *testing.T) {
	var order []string
	tt := testctx.New(t)
	tt.Use(func(next testctx.TestFunc[*testing.T]) testctx.TestFunc[*testing.T] {
		return func(ctx context.Context, t *testctx.W[*testing.T]) {
			order = append(order, "first")
			next(ctx, t)
		}
	})

	tt.Run("parent", func(ctx context.Context, t *testctx.T) {
		// Add middleware during test execution
		t.Use(func(next testctx.TestFunc[*testing.T]) testctx.TestFunc[*testing.T] {
			return func(ctx context.Context, t *testctx.W[*testing.T]) {
				order = append(order, "dynamic")
				next(ctx, t)
			}
		})

		t.Run("child", func(ctx context.Context, t *testctx.T) {
			order = append(order, "test")
		})
	})

	tt.Run("parent 2", func(ctx context.Context, t *testctx.T) {
		order = append(order, "parent 2")
	})

	// Expect first middleware to run for both parent and child,
	// dynamic middleware to run for child only
	assert.Equal(t, []string{
		"first",    // outer middleware for parent
		"first",    // outer middleware for child
		"dynamic",  // inner middleware for child
		"test",     // child test execution
		"first",    // outer middleware for parent 2
		"parent 2", // parent 2 test execution
	}, order)
}
