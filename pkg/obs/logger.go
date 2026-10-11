package obs

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// ContextHandler wraps an existing slog.Handler to inject tenant_id and trace_id from context.
type ContextHandler struct {
	slog.Handler
}

// NewContextHandler creates a handler that extracts context metadata.
func NewContextHandler(h slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: h}
}

// Handle extracts trace_id and tenant_id from the context and appends them to the record before handling.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if tenantID := TenantID(ctx); tenantID != "" {
		r.AddAttrs(slog.String("tenant_id", tenantID))
	}
	if traceID := TraceID(ctx); traceID != "" {
		r.AddAttrs(slog.String("trace_id", traceID))
	}
	return h.Handler.Handle(ctx, r)
}

// Enabled forwards the enabled check to the wrapped handler.
func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithAttrs returns a new handler with the given attributes added.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group name.
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}

// NewLogger creates an initialized JSON structured logger for a service.
func NewLogger(w io.Writer, level slog.Level, serviceName string) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	baseHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	})
	handler := NewContextHandler(baseHandler).WithAttrs([]slog.Attr{
		slog.String("service", serviceName),
	})
	return slog.New(handler)
}
