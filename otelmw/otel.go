package otelmw

import (
	"context"
	"testing"

	"dagger.io/dagger/telemetry"
)

const instrumentationLibrary = "dagger.io/testctx"

const instrumentationVersion = "v0.1.0"

var propagatedCtx = context.Background()

// Main is a helper function that initializes OTel and runs the tests
// before exiting. Use it in your TestMain function.
//
// Main covers initializing the OTel trace and logger providers, pointing
// to standard OTEL_* env vars.
//
// It also initializes a context that will be used to propagate trace
// context to subtests.
func Main(m *testing.M) int {
	propagatedCtx = telemetry.InitEmbedded(context.Background(), nil)
	exitCode := m.Run()
	telemetry.Close()
	return exitCode
}
