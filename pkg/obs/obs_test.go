package obs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qamati/IOT_V2/pkg/obs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := obs.NewLogger(&buf, slog.LevelInfo, "test-service")

	ctx := context.Background()
	ctx = obs.WithTenantID(ctx, "tenant-abc")
	ctx = obs.WithTraceID(ctx, "trace-xyz")

	logger.InfoContext(ctx, "device connected", slog.String("device_id", "dev-1"))

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "INFO", parsed["level"])
	assert.Equal(t, "test-service", parsed["service"])
	assert.Equal(t, "device connected", parsed["msg"])
	assert.Equal(t, "tenant-abc", parsed["tenant_id"])
	assert.Equal(t, "trace-xyz", parsed["trace_id"])
	assert.Equal(t, "dev-1", parsed["device_id"])
}

func TestMetricsRegistry(t *testing.T) {
	metrics := obs.NewMetrics()
	counter := metrics.NewCounter("api", "requests_total", "Total requests", []string{"method"})
	counter.WithLabelValues("POST").Inc()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "iotp_api_requests_total")
	assert.Contains(t, body, `method="POST"`)
}
