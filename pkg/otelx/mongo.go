package otelx

import (
	"context"
	"sync"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	attrDBSystem     = attribute.Key("db.system")
	attrDBName       = attribute.Key("db.name")
	attrDBOperation  = attribute.Key("db.operation")
	attrDBCollection = attribute.Key("db.mongodb.collection")
)

type mongoMonitor struct {
	spans sync.Map // requestID int64 -> trace.Span
}

// NewTracedClient 创建一个带 OTel 追踪的 Mongo Client 并注入到 go-zero
func NewTracedClient(uri string) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(uri).SetMonitor(NewMongoMonitor())
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	// 注入到 go-zero 的 mon 包中
	mon.Inject(uri, client)
	return client, nil
}

func NewMongoMonitor() *event.CommandMonitor {
	m := &mongoMonitor{}
	return &event.CommandMonitor{
		Started:   m.started,
		Succeeded: m.succeeded,
		Failed:    m.failed,
	}
}

func (m *mongoMonitor) started(ctx context.Context, e *event.CommandStartedEvent) {
	attrs := []attribute.KeyValue{
		attrDBSystem.String("mongodb"),
		attrDBName.String(e.DatabaseName),
		attrDBOperation.String(e.CommandName),
	}
	if coll := collectionFromCommand(e.CommandName, bson.Raw(e.Command)); coll != "" {
		attrs = append(attrs, attrDBCollection.String(coll))
	}
	_, span := StartSpan(ctx, "mongodb."+e.CommandName, attrs...)
	m.spans.Store(e.RequestID, span)
}

func (m *mongoMonitor) succeeded(_ context.Context, e *event.CommandSucceededEvent) {
	v, ok := m.spans.LoadAndDelete(e.RequestID)
	if !ok {
		return
	}
	v.(trace.Span).End()
}

func (m *mongoMonitor) failed(_ context.Context, e *event.CommandFailedEvent) {
	v, ok := m.spans.LoadAndDelete(e.RequestID)
	if !ok {
		return
	}
	span := v.(trace.Span)
	if e.Failure != nil {
		span.RecordError(e.Failure)
		span.SetStatus(codes.Error, e.Failure.Error())
	}
	span.End()
}

func collectionFromCommand(commandName string, command bson.Raw) string {
	val, err := command.LookupErr(commandName)
	if err != nil {
		return ""
	}
	s, ok := val.StringValueOK()
	if !ok {
		return ""
	}
	return s
}
