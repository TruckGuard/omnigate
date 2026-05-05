package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceHandler is a slog.Handler that adds trace_id and span_id to log records.
type TraceHandler struct {
	slog.Handler
}

// Handle extracts the trace context from the context and adds it to the record.
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx == nil {
		return h.Handler.Handle(ctx, r)
	}

	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.HasTraceID() {
		r.AddAttrs(slog.String("trace_id", spanContext.TraceID().String()))
	}
	if spanContext.HasSpanID() {
		r.AddAttrs(slog.String("span_id", spanContext.SpanID().String()))
	}

	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new TraceHandler with the given attributes.
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new TraceHandler with the given group name.
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}
