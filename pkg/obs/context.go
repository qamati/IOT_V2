package obs

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	tenantIDKey contextKey = "iotp.tenant_id"
	traceIDKey  contextKey = "iotp.trace_id"
)

// WithTenantID binds a tenant ID string into the context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantID extracts the tenant ID from context, or empty string if not present.
func TenantID(ctx context.Context) string {
	if val, ok := ctx.Value(tenantIDKey).(string); ok {
		return val
	}
	return ""
}

// WithTraceID explicitly binds a trace ID into context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceID extracts the trace ID from the context.
// It checks explicit trace ID in context first, then falls back to active OpenTelemetry span.
func TraceID(ctx context.Context) string {
	if val, ok := ctx.Value(traceIDKey).(string); ok && val != "" {
		return val
	}
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
