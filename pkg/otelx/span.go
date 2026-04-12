package otelx

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "psych-core-api"
)

func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

func StartSpan(ctx context.Context, spanName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, spanName, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindInternal))
}

func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
