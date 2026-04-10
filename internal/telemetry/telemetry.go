// Package telemetry provides OpenTelemetry tracing support for axe.
//
// Tracing is enabled when OTEL_EXPORTER_OTLP_ENDPOINT is set; when the
// variable is absent the package installs a no-op tracer so callers pay
// zero overhead and never crash.
package telemetry

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const tracerName = "github.com/jrswab/axe"

// Shutdown is a function that flushes and shuts down the tracer provider.
type Shutdown func(ctx context.Context) error

// Init initialises the global tracer. If OTEL_EXPORTER_OTLP_ENDPOINT is not
// set it installs a no-op tracer and returns a no-op shutdown function. The
// caller must call shutdown before the process exits to flush pending spans.
func Init(ctx context.Context) (Shutdown, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(_ context.Context) error { return nil }, nil
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "axe"
	}

	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		// Fall back to no-op rather than crashing the agent.
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(_ context.Context) error { return nil }, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithProcessPID(),
	)
	if err != nil {
		// Non-fatal; use a minimal resource.
		res = resource.Default()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(shutdownCtx)
	}
	return shutdown, nil
}

// Tracer returns the package-level tracer.
func Tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer(tracerName)
}

// RecordError marks a span as failed and records the error.
func RecordError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
