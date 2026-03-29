package platform

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// InitObservability sets up the TracerProvider, MeterProvider, and log-trace correlation.
// Returns a cleanup function that flushes pending spans on shutdown.
func InitObservability(ctx context.Context, serviceName, otlpEndpoint string) (shutdown func(context.Context) error, err error) {
	var shutdowns []func(context.Context) error

	shutdown = func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdowns {
			if err := fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	// Set W3C Trace Context propagator for trace ID propagation.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Tracer provider with OTLP gRPC exporter.
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return shutdown, fmt.Errorf("init trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(newResource(serviceName)),
	)
	otel.SetTracerProvider(tp)
	shutdowns = append(shutdowns, tp.Shutdown)

	// Meter provider with Prometheus exporter.
	promExporter, err := promexporter.New()
	if err != nil {
		return shutdown, fmt.Errorf("init prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithResource(newResource(serviceName)),
	)
	otel.SetMeterProvider(mp)
	shutdowns = append(shutdowns, mp.Shutdown)

	return shutdown, nil
}

// TraceIDFromContext extracts the trace ID string from a span context.
// Returns "" if no active span.
func TraceIDFromContext(ctx context.Context) string {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
