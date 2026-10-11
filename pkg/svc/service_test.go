package svc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/pkg/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceLifecycle(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	baseURL := "http://" + ln.Addr().String()

	s := svc.New("test-core", svc.Config{
		ShutdownTimeout: 2 * time.Second,
	}, nil)
	s.SetListener(ln)

	var dbReady bool
	s.RegisterReadinessCheck("database", func(ctx context.Context) error {
		if !dbReady {
			return errors.New("db connection not established")
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.Run(ctx)
	}()

	// Give server time to bind and listen
	time.Sleep(50 * time.Millisecond)

	// 1. Healthz check
	resp, err := http.Get(baseURL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var healthData map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&healthData))
	assert.Equal(t, "UP", healthData["status"])
	assert.Equal(t, "test-core", healthData["service"])

	// 2. Readyz check when not ready
	respReady, err := http.Get(baseURL + "/readyz")
	require.NoError(t, err)
	defer respReady.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, respReady.StatusCode)

	// 3. Mark DB as ready and recheck
	dbReady = true
	respReady2, err := http.Get(baseURL + "/readyz")
	require.NoError(t, err)
	defer respReady2.Body.Close()
	assert.Equal(t, http.StatusOK, respReady2.StatusCode)

	// 4. Metrics endpoint
	respMetrics, err := http.Get(baseURL + "/metrics")
	require.NoError(t, err)
	defer respMetrics.Body.Close()
	assert.Equal(t, http.StatusOK, respMetrics.StatusCode)

	// 5. Cancel context and verify graceful shutdown
	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("service did not shut down in time")
	}
}
