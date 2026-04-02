package oteltest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/dagger/testctx"
	"github.com/dagger/testctx/oteltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// spanResult holds serialized span data for cross-process verification.
type spanResult struct {
	Name       string `json:"name"`
	StatusCode int    `json:"status_code"`
	StatusDesc string `json:"status_desc"`
}

// TestSubprocess is a helper that only runs when invoked as a subprocess.
// It sets up a span recorder, runs failing subtests, and writes span data
// to the file specified by OTELTEST_SPAN_FILE. Individual subtests are
// selected via -test.run filtering.
func TestSubprocess(t *testing.T) {
	spanFile := os.Getenv("OTELTEST_SPAN_FILE")
	if spanFile == "" {
		t.Skip("only run as subprocess")
	}

	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))

	tt := testctx.New(t, oteltest.WithTracing(oteltest.TraceConfig[*testing.T]{
		TracerProvider: tracerProvider,
	}))

	t.Cleanup(func() {
		spans := spanRecorder.Ended()
		var results []spanResult
		for _, s := range spans {
			results = append(results, spanResult{
				Name:       s.Name(),
				StatusCode: int(s.Status().Code),
				StatusDesc: s.Status().Description,
			})
		}
		data, _ := json.Marshal(results)
		os.WriteFile(spanFile, data, 0644)
	})

	tt.Run("SingleError", func(ctx context.Context, t *testctx.T) {
		t.Error("something went wrong")
	})

	tt.Run("MultipleErrors", func(ctx context.Context, t *testctx.T) {
		t.Error("first error")
		t.Errorf("second error: %d", 42)
	})

	tt.Run("Fatal", func(ctx context.Context, t *testctx.T) {
		t.Fatal("fatal error")
	})

	tt.Run("Fatalf", func(ctx context.Context, t *testctx.T) {
		t.Fatalf("fatal: %s", "boom")
	})

	tt.Run("ErrorThenFatal", func(ctx context.Context, t *testctx.T) {
		t.Error("non-fatal")
		t.Fatal("fatal")
	})

	tt.Run("FailNoMessage", func(ctx context.Context, t *testctx.T) {
		t.Fail()
	})

	tt.Run("TestifyRequireNoError", func(ctx context.Context, t *testctx.T) {
		require.NoError(t, fmt.Errorf("something failed"))
	})

	tt.Run("ParallelChildFails", func(ctx context.Context, t *testctx.T) {
		t.Run("passing", func(ctx context.Context, t *testctx.T) {
			t.Unwrap().Parallel()
		})
		t.Run("failing", func(ctx context.Context, t *testctx.T) {
			t.Unwrap().Parallel()
			t.Error("child failed")
		})
	})
}

// runSubprocess invokes the test binary as a subprocess, selecting a specific
// TestSubprocess subtest, and returns the recorded span data.
func runSubprocess(t testing.TB, subtest string) []spanResult {
	t.Helper()

	f, err := os.CreateTemp("", "oteltest-spans-*.json")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	cmd := exec.Command(os.Args[0],
		"-test.run=^TestSubprocess$/^"+subtest+"$",
		"-test.count=1",
	)
	cmd.Env = append(os.Environ(), "OTELTEST_SPAN_FILE="+f.Name())
	cmd.Run() // ignore exit code; the subprocess test is expected to fail

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)

	var results []spanResult
	require.NoError(t, json.Unmarshal(data, &results))
	return results
}

func TestFailedTestErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		subtest  string
		wantDesc string // exact match (when set)
		check    func(t *testing.T, spans []spanResult)
	}{
		{
			name:     "single Error populates span status",
			subtest:  "SingleError",
			wantDesc: "something went wrong",
		},
		{
			name:     "multiple errors joined with newlines",
			subtest:  "MultipleErrors",
			wantDesc: "first error\nsecond error: 42",
		},
		{
			name:     "Fatal populates span status",
			subtest:  "Fatal",
			wantDesc: "fatal error",
		},
		{
			name:     "Fatalf populates span status",
			subtest:  "Fatalf",
			wantDesc: "fatal: boom",
		},
		{
			name:     "Error then Fatal accumulates both",
			subtest:  "ErrorThenFatal",
			wantDesc: "non-fatal\nfatal",
		},
		{
			name:     "Fail with no message falls back to test failed",
			subtest:  "FailNoMessage",
			wantDesc: "test failed",
		},
		{
			name:     "testify verbose error is cleaned",
			subtest:  "TestifyRequireNoError",
			wantDesc: "Received unexpected error:\nsomething failed",
		},
		{
			name:    "parallel child failure reflects on parent",
			subtest: "ParallelChildFails",
			check: func(t *testing.T, spans []spanResult) {
				// Expect 3 spans: passing, failing, and the parent
				require.Len(t, spans, 3)
				byName := map[string]spanResult{}
				for _, s := range spans {
					byName[s.Name] = s
				}
				// The parent span must reflect the child failure
				parent := byName["ParallelChildFails"]
				assert.Equal(t, int(codes.Error), parent.StatusCode,
					"parent span should be marked as failed")
				assert.Equal(t, "child failed", parent.StatusDesc)
				// The failing child should have the error message
				failing := byName["failing"]
				assert.Equal(t, int(codes.Error), failing.StatusCode)
				assert.Equal(t, "child failed", failing.StatusDesc)
				// The passing child should be OK
				passing := byName["passing"]
				assert.Equal(t, int(codes.Ok), passing.StatusCode)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spans := runSubprocess(t, tc.subtest)
			if tc.check != nil {
				tc.check(t, spans)
			} else {
				require.Len(t, spans, 1)
				assert.Equal(t, int(codes.Error), spans[0].StatusCode)
				assert.Equal(t, tc.wantDesc, spans[0].StatusDesc)
			}
		})
	}
}
