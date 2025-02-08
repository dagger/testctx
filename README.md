# testctx

Package `testctx` extends Go's `testing` package with support for `context.Context` and middleware.

The main benefits are:

- **Context Support**: Tests and benchmarks receive a `context.Context` that's automatically canceled when they complete
- **Middleware**: Add behaviors like timeouts, tracing, or parallel execution across entire test suites
- **Composable**: Mix and match middleware to build the exact test environment you need
- **Go-native**: Works seamlessly with existing Go test patterns and tools (no custom CLI, etc.)

## Example

```go
type MySuite struct{}

func (t *MySuite) TestExample(ctx context.Context, t *testctx.T) {
    // ...
}

func (t *MySuite) BenchmarkExample(ctx context.Context, b *testctx.B) {
    // ...
}

func TestAll(t *testing.T) {
	testctx.New(t,
		testctx.WithTimeout(time.Minute), // each test has a 1 minute timeout
		testctx.WithParallel(), // run tests in parallel
	).RunTests(&MySuite{}) // runs TestExample
}

func BenchAll(t *testing.T) {
	testctx.New(t,
		testctx.WithTimeout(10 * time.Minute), // each benchmark has a 10 minute timeout
	).RunBenchmarks(&MySuite{}) // runs BenchmarkExample
}
```

The `oteltest` package provides middleware for transparent tracing:

```go
package oteltest_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dagger/testctx"
	"github.com/dagger/testctx/oteltest"
)

func TestMain(m *testing.M) {
	os.Exit(oteltest.Main(m)) // auto-wires OTel exporters
}

func TestFruits(t *testing.T) {
	testctx.New(t,
		testctx.WithParallel(), // run tests in parallel
		oteltest.WithTracing[*testing.T](), // trace each test and subtest
		oteltest.WithLogging[*testing.T](), // direct t.Log etc. to span logs
	).RunTests(&FruitSuite{})
}

type FruitSuite struct{}

func (FruitSuite) TestApple(ctx context.Context, t *testctx.T) {
    // ...
}

func (FruitSuite) TestBanana(ctx context.Context, t *testctx.T) {
    // ...
}

// Creates the following span tree:
//
//	TestFruits
//	├── TestApple
//	└── TestBanana
//
// All test logs are additionally sent to OpenTelemetry if configured.
```

## Middleware

Middleware are implemented as functions that wrap test functions.

```go
// WithTimeout creates middleware that adds a timeout to the test context
func WithTimeout[T testctx.Runner[T]](d time.Duration) testctx.Middleware[T] {
	return func(next testctx.RunFunc[T]) testctx.RunFunc[T] {
		return func(ctx context.Context, t *testctx.W[T]) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			next(ctx, t)
		}
	}
}
```

The types look a little spooky, but it's so that they can work with both `*testing.T` and `*testing.B` without extra type assertions.

## Differences from Go 1.24 `t.Context()`

Go 1.24 adds a `t.Context()` accessor which provides a `context.Context` that's canceled when the test completes. However, it doesn't provide any way to modify that context.

This package goes further by:
- Passing context directly to test methods
- Supporting `WithContext()` to modify contexts for subtests
- Adding middleware support for transparent instrumentation