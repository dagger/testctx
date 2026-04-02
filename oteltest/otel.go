package oteltest

import (
	"context"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/dagger/otel-go"
)

const instrumentationLibrary = "dagger.io/testctx"

const instrumentationVersion = "v0.1.0"

var propagatedCtx = context.Background()

// testPackage is the import path of the package under test, detected from
// the test binary's build info (e.g. "example.com/project/pkg").
var testPackage string

// Main is a helper function that initializes OTel and runs the tests
// before exiting. Use it in your TestMain function.
//
// Main covers initializing the OTel trace and logger providers, pointing
// to standard OTEL_* env vars.
//
// It also initializes a context that will be used to propagate trace
// context to subtests.
func Main(m *testing.M) int {
	propagatedCtx = otel.InitEmbedded(context.Background(), nil)
	testPackage = detectTestPackage()
	exitCode := m.Run()
	otel.Close()
	return exitCode
}

// detectTestPackage returns the import path of the package under test by
// reading the test binary's build info. Go test binaries have a Path like
// "example.com/project/pkg.test"; we strip the ".test" suffix.
func detectTestPackage() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return strings.TrimSuffix(bi.Path, ".test")
}
