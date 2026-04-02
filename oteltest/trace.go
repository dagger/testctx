package oteltest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/dagger/testctx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
)

// TraceConfig holds configuration for the OpenTelemetry tracing middleware
type TraceConfig[T testctx.Runner[T]] struct {
	// TracerProvider to use for creating spans. If nil, the global provider will be used.
	TracerProvider trace.TracerProvider
	// Attributes to add to all test spans
	Attributes []attribute.KeyValue
	// StartOptions allows customizing the span start options for each test/benchmark
	StartOptions func(*testctx.W[T]) []trace.SpanStartOption
}

// testSpanKey is the key used to store the test span in the context
type testSpanKey struct{}

// WithTracing creates middleware that adds OpenTelemetry tracing around each test/benchmark
func WithTracing[T testctx.Runner[T]](cfg ...TraceConfig[T]) testctx.Middleware[T] {
	var c TraceConfig[T]
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.TracerProvider == nil {
		c.TracerProvider = otel.GetTracerProvider()
	}

	tracer := c.TracerProvider.Tracer(
		instrumentationLibrary,
		trace.WithInstrumentationVersion(instrumentationVersion),
	)

	return func(next testctx.RunFunc[T]) testctx.RunFunc[T] {
		return func(ctx context.Context, w *testctx.W[T]) {
			// Inherit from any trace context that Main picked up
			if !trace.SpanContextFromContext(ctx).IsValid() {
				ctx = trace.ContextWithSpanContext(ctx, trace.SpanContextFromContext(propagatedCtx))
			}

			// Start a new span for this test/benchmark
			attrs := []attribute.KeyValue{
				semconv.TestCaseName(w.Name()),
			}
			if testPackage != "" {
				attrs = append(attrs, semconv.TestSuiteName(testPackage))
			}
			opts := []trace.SpanStartOption{
				trace.WithAttributes(attrs...),
				trace.WithAttributes(c.Attributes...),
			}

			// Link to the parent test span so that tools can attribute the subtest
			// runtime to the parent test when tests are run in parallel
			if val, ok := ctx.Value(testSpanKey{}).(trace.Span); ok {
				opts = append(opts, trace.WithLinks(trace.Link{
					SpanContext: val.SpanContext(),
				}))
			}

			if c.StartOptions != nil {
				opts = append(opts, c.StartOptions(w)...)
			}

			spanName := w.BaseName()

			// Accumulate Error/Fatal messages so the span status carries the
			// actual failure reason instead of a generic "test failed".
			errorsAcc := &errorAccumulator{}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer func() {
				var testStatus attribute.KeyValue
				if ctx.Err() != nil {
					// Test was interrupted (timeout or cancellation)
					span.SetStatus(codes.Error, "test interrupted: "+ctx.Err().Error())
					if errors.Is(ctx.Err(), context.DeadlineExceeded) {
						testStatus = semconv.TestSuiteRunStatusTimedOut
					} else {
						testStatus = semconv.TestSuiteRunStatusAborted
					}
				} else if w.Failed() {
					desc := errorsAcc.String()
					if desc == "" {
						desc = "test failed"
					}
					span.SetStatus(codes.Error, desc)
					testStatus = semconv.TestSuiteRunStatusFailure
				} else if w.Skipped() {
					span.SetStatus(codes.Ok, "test skipped")
					testStatus = semconv.TestSuiteRunStatusSkipped
				} else {
					span.SetStatus(codes.Ok, "test passed")
					testStatus = semconv.TestSuiteRunStatusSuccess
				}
				span.SetAttributes(testStatus)
				span.End()
			}()

			// Store the span in the context so that it can be linked to in subtests
			ctx = context.WithValue(ctx, testSpanKey{}, span)

			next(ctx, w.WithLogger(errorsAcc))
		}
	}
}

// errorAccumulator is a Logger that captures Error/Errorf messages so they
// can be attached to a span status when the test fails.
type errorAccumulator struct {
	mu       sync.Mutex
	messages []string
}

var _ testctx.Logger = (*errorAccumulator)(nil)

func (a *errorAccumulator) Log(args ...any)                 {}
func (a *errorAccumulator) Logf(format string, args ...any) {}

func (a *errorAccumulator) Error(args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, cleanErrorMessage(fmt.Sprint(args...)))
}

func (a *errorAccumulator) Errorf(format string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, cleanErrorMessage(fmt.Sprintf(format, args...)))
}

// String returns all accumulated error messages joined by newlines.
func (a *errorAccumulator) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.Join(a.messages, "\n")
}

// cleanErrorMessage extracts the meaningful error content from verbose test
// failure messages like those produced by testify's assert/require packages.
// It strips the "Error Trace:" and "Test:" sections, keeping only "Error:"
// and "Messages:" content. If the message doesn't match the expected format,
// it is returned unchanged (with leading/trailing whitespace trimmed).
func cleanErrorMessage(msg string) string {
	// Quick check: does this look like a testify-formatted message?
	if !strings.Contains(msg, "\tError:") {
		return strings.TrimSpace(msg)
	}

	lines := strings.Split(msg, "\n")
	var result []string
	inWanted := false
	found := false

	for _, line := range lines {
		// Section headers look like: \t<Name>:<padding>\t<value>
		// They start with \t followed by a non-whitespace character.
		if len(line) > 1 && line[0] == '\t' && line[1] != ' ' && line[1] != '\t' {
			inWanted = false
			rest := line[1:]
			colonIdx := strings.Index(rest, ":")
			if colonIdx < 0 {
				continue
			}
			name := rest[:colonIdx]
			after := strings.TrimLeft(rest[colonIdx+1:], " ")
			if len(after) == 0 || after[0] != '\t' {
				continue
			}
			if name == "Error" || name == "Messages" {
				inWanted = true
				found = true
				if v := strings.TrimSpace(after[1:]); v != "" {
					result = append(result, v)
				}
			}
			continue
		}

		// Continuation lines look like: \t<spaces>\t<value>
		if inWanted && len(line) > 0 && line[0] == '\t' {
			rest := strings.TrimLeft(line[1:], " ")
			if len(rest) > 0 && rest[0] == '\t' {
				if v := strings.TrimSpace(rest[1:]); v != "" {
					result = append(result, v)
				}
			}
		}
	}

	if found && len(result) > 0 {
		return strings.Join(result, "\n")
	}
	return strings.TrimSpace(msg)
}
