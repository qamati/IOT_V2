package modbus

import (
	"context"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/pkg/connectors"
)

type mockSink struct {
	telemetryCount int
	lastValues     map[string]any
	connectedDevs  []string
}

func (m *mockSink) Telemetry(dev string, ts int64, values map[string]any) {
	m.telemetryCount++
	m.lastValues = values
}

func (m *mockSink) Attributes(dev string, values map[string]any) {}

func (m *mockSink) Connected(dev string, meta map[string]string) {
	m.connectedDevs = append(m.connectedDevs, dev)
}

func (m *mockSink) Disconnected(dev string) {}

func TestModbusConnector_PollAndWrite(t *testing.T) {
	c := NewConnector("modbus-line-1")
	if err := c.Configure(nil); err != nil {
		t.Fatalf("configure failed: %v", err)
	}

	sink := &mockSink{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = c.Run(ctx, sink)
	}()

	// Wait for initial connection & first poll tick
	time.Sleep(1200 * time.Millisecond)

	if len(sink.connectedDevs) == 0 {
		t.Errorf("expected connected devices")
	}
	if sink.telemetryCount == 0 {
		t.Fatalf("expected at least 1 telemetry poll")
	}

	// Verify telemetry keys
	vals := sink.lastValues
	if _, ok := vals["motor_speed_rpm"]; !ok {
		t.Errorf("expected motor_speed_rpm in telemetry, got %v", vals)
	}

	// Test write command (SBO / RPC method)
	resp, err := c.HandleWrite(context.Background(), connectors.WriteReq{
		Device: "modbus-pump-station-01",
		Method: "setSpeed",
		Params: map[string]any{"rpm": 1800.0, "address": 100.0},
	})
	if err != nil || !resp.Success {
		t.Fatalf("write failed: %v, resp=%+v", err, resp)
	}

	// Health check
	h := c.Health()
	if h.Status != "HEALTHY" || h.PollsCount == 0 {
		t.Errorf("unexpected health: %+v", h)
	}
}
