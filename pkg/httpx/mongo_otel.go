package httpx

import (
	"context"
	"sync"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	mongoTracer = otel.Tracer("mongo-driver")
)

// NewTracedClient 创建一个带 OTel 追踪的 Mongo Client 并注入到 go-zero
func NewTracedClient(uri string) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(uri).SetMonitor(NewMongoMonitor())
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	// 注入到 go-zero 的 mon 包中，这样 MustNewModel 就会使用这个 client
	mon.Inject(uri, client)
	return client, nil
}

type MongoMonitor struct {
	spans sync.Map
}

func NewMongoMonitor() *event.CommandMonitor {
	m := &MongoMonitor{}
	return &event.CommandMonitor{
		Started:   m.Started,
		Succeeded: m.Succeeded,
		Failed:    m.Failed,
	}
}

func (m *MongoMonitor) Started(ctx context.Context, evt *event.CommandStartedEvent) {
	_, span := mongoTracer.Start(ctx, "mongo."+evt.CommandName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "mongodb"),
			attribute.String("db.name", evt.DatabaseName),
			attribute.String("db.operation", evt.CommandName),
			attribute.String("db.mongodb.collection", evt.CommandName), // Simplified
		),
	)
	m.spans.Store(evt.RequestID, span)
}

func (m *MongoMonitor) Succeeded(ctx context.Context, evt *event.CommandSucceededEvent) {
	if s, ok := m.spans.LoadAndDelete(evt.RequestID); ok {
		span := s.(trace.Span)
		span.End()
	}
}

func (m *MongoMonitor) Failed(ctx context.Context, evt *event.CommandFailedEvent) {
	if s, ok := m.spans.LoadAndDelete(evt.RequestID); ok {
		span := s.(trace.Span)
		span.RecordError(evt.Failure)
		span.SetStatus(codes.Error, evt.Failure.Error())
		span.End()
	}
}
