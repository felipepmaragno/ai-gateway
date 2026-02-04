package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func Init(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	if otlpEndpoint == "" {
		tracer = otel.Tracer(serviceName)
		slog.Info("telemetry disabled, no OTLP endpoint configured")
		return func(ctx context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("0.3.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = tp.Tracer(serviceName)

	slog.Info("telemetry initialized", "endpoint", otlpEndpoint)

	return tp.Shutdown, nil
}

func Tracer() trace.Tracer {
	if tracer == nil {
		tracer = otel.Tracer("ai-gateway")
	}
	return tracer
}

func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

func AddRequestAttributes(span trace.Span, tenantID, provider, model, requestID string) {
	span.SetAttributes(
		attribute.String("tenant.id", tenantID),
		attribute.String("provider", provider),
		attribute.String("model", model),
		attribute.String("request.id", requestID),
	)
}

func AddTokenAttributes(span trace.Span, inputTokens, outputTokens int) {
	span.SetAttributes(
		attribute.Int("tokens.input", inputTokens),
		attribute.Int("tokens.output", outputTokens),
		attribute.Int("tokens.total", inputTokens+outputTokens),
	)
}

func AddCostAttribute(span trace.Span, costUSD float64) {
	span.SetAttributes(
		attribute.Float64("cost.usd", costUSD),
	)
}

func AddCacheAttribute(span trace.Span, cacheHit bool) {
	span.SetAttributes(
		attribute.Bool("cache.hit", cacheHit),
	)
}

func AddErrorAttribute(span trace.Span, err error) {
	span.SetAttributes(
		attribute.String("error.message", err.Error()),
	)
	span.RecordError(err)
}

func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
