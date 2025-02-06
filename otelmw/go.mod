module github.com/dagger/testctx/otelmw

go 1.23

require (
	github.com/dagger/testctx v0.1.0
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/trace v1.24.0
)

require (
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
)

replace github.com/dagger/testctx => ../
