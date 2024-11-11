package nagaya

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	KeyTenant    = attribute.Key("nagaya.tenant")
	KeyRequestID = attribute.Key("nagaya.request_id")
)

func getTracer(tracerProvider trace.TracerProvider) trace.Tracer {
	tp := tracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	return tp.Tracer("github.com/aereal/nagaya.Nagaya")
}

func finishSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func attrRequestID(reqID string) attribute.KeyValue { return KeyRequestID.String(reqID) }

func attrTenant(tenant Tenant) attribute.KeyValue { return KeyTenant.String(string(tenant)) }
