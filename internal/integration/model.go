package integration

import (
	"encoding/json"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// Converter direction
const (
	DirectionUplink   = "UPLINK"
	DirectionDownlink = "DOWNLINK"
)

// Converter modes
const (
	ModeDeclarative = "DECLARATIVE"
	ModeExpression  = "EXPRESSION"
)

// Integration types
const (
	TypeHTTPWebhook       = "HTTP_WEBHOOK"
	TypeMQTTClient        = "MQTT_CLIENT"
	TypeLoRaWANChirpStack = "LORAWAN_CHIRPSTACK"
	TypeTTN               = "TTN"
	TypeOPCUA             = "OPC_UA"
)

// TestCase represents a saved unit test for a converter.
type TestCase struct {
	Name           string            `json:"name"`
	Payload        string            `json:"payload"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	ExpectedOutput map[string]any    `json:"expected_output,omitempty"`
}

// Converter defines payload transformation rules.
type Converter struct {
	ID         ids.ID          `json:"id"`
	TenantID   ids.ID          `json:"tenant_id"`
	Name       string          `json:"name"`
	Direction  string          `json:"direction"` // UPLINK | DOWNLINK
	Mode       string          `json:"mode"`      // DECLARATIVE | EXPRESSION
	Definition json.RawMessage `json:"definition"`
	Tests      []TestCase      `json:"tests,omitempty"`
	Version    int             `json:"version"`
}

// Integration defines an external protocol connector instance.
type Integration struct {
	ID                  ids.ID          `json:"id"`
	TenantID            ids.ID          `json:"tenant_id"`
	Name                string          `json:"name"`
	Type                string          `json:"type"` // HTTP_WEBHOOK | MQTT_CLIENT | LORAWAN_CHIRPSTACK ...
	Enabled             bool            `json:"enabled"`
	UplinkConverterID   ids.ID          `json:"uplink_converter_id"`
	DownlinkConverterID ids.ID          `json:"downlink_converter_id,omitempty"`
	RoutingKey          string          `json:"routing_key"`
	Config              json.RawMessage `json:"config"`
	AllowCreateDevice   bool            `json:"allow_create_device"`
	DefaultDeviceType   string          `json:"default_device_type"`
	Version             int             `json:"version"`
}

// IntegrationStatus tracks runtime health and telemetry counters.
type IntegrationStatus struct {
	IntegrationID ids.ID           `json:"integration_id"`
	State         string           `json:"state"` // RUNNING | STOPPED | ERROR
	LastError     string           `json:"last_error,omitempty"`
	LastOkTs      int64            `json:"last_ok_ts"`
	Counters      map[string]int64 `json:"counters"` // rx_events, tx_events, errors, auto_created
	Instance      string           `json:"instance"`
}

// DeviceRef references or identifies an entity in an uplink event.
type DeviceRef struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Label     string `json:"label,omitempty"`
	IsGateway bool   `json:"is_gateway,omitempty"`
}

// TsBatch is a batch of values timestamped at epoch ms.
type TsBatch struct {
	Ts     int64          `json:"ts"`
	Values map[string]any `json:"values"`
}

// UplinkEvent is the platform canonical decoded event.
type UplinkEvent struct {
	Device     DeviceRef         `json:"device"`
	Telemetry  []TsBatch         `json:"telemetry"`
	Attributes map[string]any    `json:"attributes,omitempty"` // client scope
	Server     map[string]any    `json:"server,omitempty"`     // server scope
	Meta       map[string]string `json:"meta,omitempty"`
	Raw        string            `json:"raw,omitempty"`
}

// DownlinkEvent represents a command or attribute update destined for an external system.
type DownlinkEvent struct {
	Device DeviceRef      `json:"device"`
	Kind   string         `json:"kind"` // RPC | ATTR_UPDATE | ADMIN
	Method string         `json:"method"`
	Params any            `json:"params"`
	ID     string         `json:"id"`
}

// DebugEvent holds a captured trace of a recent payload conversion.
type DebugEvent struct {
	ID        ids.ID            `json:"id"`
	Ts        int64             `json:"ts"`
	Direction string            `json:"direction"`
	Payload   string            `json:"payload"`
	Metadata  map[string]string `json:"metadata"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Result    *UplinkEvent      `json:"result,omitempty"`
}
