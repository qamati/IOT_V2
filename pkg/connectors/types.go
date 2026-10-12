package connectors

import (
	"context"
	"encoding/json"
)

// Sink consumes samples and device status updates from connectors.
type Sink interface {
	Telemetry(dev string, ts int64, values map[string]any)
	Attributes(dev string, values map[string]any)
	Connected(dev string, meta map[string]string)
	Disconnected(dev string)
}

// WriteReq represents a command or write request destined for a field device.
type WriteReq struct {
	Device string         `json:"device"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// WriteResp represents the result of a field device write.
type WriteResp struct {
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	Values  map[string]any `json:"values,omitempty"`
}

// DeviceInfo describes a child device managed by a connector.
type DeviceInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
}

// Health describes runtime health and statistics of a connector.
type Health struct {
	Status      string `json:"status"` // HEALTHY | DEGRADED | UNHEALTHY
	LastError   string `json:"last_error,omitempty"`
	PollsCount  int64  `json:"polls_count"`
	ErrorsCount int64  `json:"errors_count"`
	UptimeSec   int64  `json:"uptime_sec"`
}

// Connector defines the SPI implemented by industrial and field connectors.
type Connector interface {
	Name() string
	Type() string
	Configure(cfg json.RawMessage) error
	Run(ctx context.Context, sink Sink) error
	HandleWrite(ctx context.Context, w WriteReq) (WriteResp, error)
	Devices() []DeviceInfo
	Health() Health
}
