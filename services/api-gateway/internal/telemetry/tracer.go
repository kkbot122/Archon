package telemetry

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitTracer sets up a basic OpenTelemetry provider
// Note: To send this to Jaeger, you would add an OTLP exporter here.
func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		// sdktrace.WithBatcher(exporter), <-- Add Jaeger OTLP exporter here in the future
	)
	
	// Set the global Tracer Provider
	otel.SetTracerProvider(tp)
	log.Println("✅ OpenTelemetry Tracer Initialized")
	
	return tp, nil
}