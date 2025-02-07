package otelmw

import (
	"context"
	"fmt"

	"dagger.io/dagger/telemetry"
	"github.com/dagger/testctx"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// LogConfig holds configuration for the OpenTelemetry logging middleware
type LogConfig struct {
	// LoggerProvider to use for logging. If nil, the global provider will be used.
	LoggerProvider *sdklog.LoggerProvider
}

// WithLogging creates middleware that adds OpenTelemetry logging to each test/benchmark
func WithLogging[T testctx.Runner[T]](cfg ...LogConfig) testctx.Middleware[T] {
	var c LogConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.LoggerProvider == nil {
		c.LoggerProvider = telemetry.LoggerProvider(propagatedCtx)
	}

	return func(next testctx.RunFunc[T]) testctx.RunFunc[T] {
		return func(ctx context.Context, w *testctx.W[T]) {
			// Use the same logger provider as the main test
			ctx = telemetry.WithLoggerProvider(ctx, c.LoggerProvider)

			// Send logs to the span
			next(ctx, w.WithLogger(&spanLogger{
				streams: telemetry.SpanStdio(ctx, instrumentationLibrary),
			}))
		}
	}
}

type spanLogger struct {
	streams telemetry.SpanStreams
}

func (l *spanLogger) Log(args ...any) {
	fmt.Fprintln(l.streams.Stdout, args...)
}

func (l *spanLogger) Logf(format string, args ...any) {
	fmt.Fprintf(l.streams.Stdout, format+"\n", args...)
}

func (l *spanLogger) Error(args ...any) {
	fmt.Fprintln(l.streams.Stderr, args...)
}

func (l *spanLogger) Errorf(format string, args ...any) {
	fmt.Fprintf(l.streams.Stderr, format+"\n", args...)
}
