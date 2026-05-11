package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const endpointEnvVar = "OTEL_EXPORTER_OTLP_ENDPOINT"

// ShutdownFunc flushes and shuts down telemetry providers. Safe to call when telemetry is disabled.
type ShutdownFunc func(context.Context) error

// Init configures the global OpenTelemetry MeterProvider when OTEL_EXPORTER_OTLP_ENDPOINT is set.
// When the env var is empty, telemetry stays disabled and the returned shutdown is a no-op.
func Init(ctx context.Context, serviceName string) (ShutdownFunc, bool, error) {
	if os.Getenv(endpointEnvVar) == "" {
		return func(context.Context) error { return nil }, false, nil
	}

	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, false, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, false, err
	}

	provider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(exporter)),
	)
	otel.SetMeterProvider(provider)

	return provider.Shutdown, true, nil
}
