package scada

import (
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Tag Qualities
const (
	QualityGood      = "GOOD"      // 192 (0xC0)
	QualityBad       = "BAD"       // 0   (0x00)
	QualityUncertain = "UNCERTAIN" // 64  (0x40)
)

// Tag Data Types
const (
	DataTypeAnalog   = "ANALOG"
	DataTypeDiscrete = "DISCRETE"
	DataTypeString   = "STRING"
)

// ISA-18.2 Alarm States
const (
	AlarmStateNormal       = "NORMAL"
	AlarmStateUnackAlarm   = "UNACK_ALARM"
	AlarmStateAckedAlarm   = "ACKED_ALARM"
	AlarmStateRtnUnack     = "RTN_UNACK"
	AlarmStateShelved      = "SHELVED"
	AlarmStateSuppressed   = "SUPPRESSED"
	AlarmStateOutOfService = "OUT_OF_SERVICE"
)

// Tag represents an industrial process tag point.
type Tag struct {
	ID           ids.ID  `json:"id"`
	TenantID     ids.ID  `json:"tenant_id"`
	DeviceID     ids.ID  `json:"device_id,omitempty"`
	Path         string  `json:"path"` // Hierarchical dot notation, e.g. "Plant.Area1.Pump101.Flow"
	DataType     string  `json:"data_type"` // ANALOG | DISCRETE | STRING
	EngUnit      string  `json:"eng_unit,omitempty"` // e.g. "GPM", "bar", "°C", "RPM"
	Deadband     float64 `json:"deadband,omitempty"` // Change required to emit telemetry update
	ScaleFactor  float64 `json:"scale_factor,omitempty"`
	ScaleOffset  float64 `json:"scale_offset,omitempty"`
	EUMin        float64 `json:"eu_min,omitempty"`
	EUMax        float64 `json:"eu_max,omitempty"`
	ScanRateMs   int     `json:"scan_rate_ms"`

	// Current live state
	Value        any     `json:"value"`
	LastValue    any     `json:"last_value,omitempty"`
	Quality      string  `json:"quality"`
	Timestamp    int64   `json:"timestamp"`
	Version      int     `json:"version"`
}

// ISA182Alarm tracks an industrial process alarm under the ISA-18.2 standard state machine.
type ISA182Alarm struct {
	ID           ids.ID  `json:"id"`
	TenantID     ids.ID  `json:"tenant_id"`
	TagPath      string  `json:"tag_path"`
	AlarmType    string  `json:"alarm_type"` // HIGH_HIGH | HIGH | LOW | LOW_LOW | DEV
	Setpoint     float64 `json:"setpoint"`
	Severity     string  `json:"severity"` // CRITICAL | HIGH | MEDIUM | LOW
	State        string  `json:"state"`    // NORMAL | UNACK_ALARM | ACKED_ALARM | RTN_UNACK | SHELVED ...
	ActiveTs     int64   `json:"active_ts"`
	AckTs        int64   `json:"ack_ts,omitempty"`
	AckUser      string  `json:"ack_user,omitempty"`
	ShelveUntil  int64   `json:"shelve_until,omitempty"`
	Message      string  `json:"message"`
}

// ControlCommand represents a Select-Before-Operate (SBO) safe industrial control invocation.
type ControlCommand struct {
	SelectID     ids.ID  `json:"select_id"`
	TenantID     ids.ID  `json:"tenant_id"`
	TagPath      string  `json:"tag_path"`
	DesiredValue any     `json:"desired_value"`
	Operator     string  `json:"operator"`
	SelectedAt   int64   `json:"selected_at"`
	OperatedAt   int64   `json:"operated_at,omitempty"`
	Status       string  `json:"status"` // SELECTED | OPERATED | CANCELLED | INTERLOCK_DENIED
	Reason       string  `json:"reason,omitempty"`
}

// ControlLogEntry represents an audited historical Select-Before-Operate control action.
type ControlLogEntry struct {
	ID           ids.ID  `json:"id"`
	TenantID     ids.ID  `json:"tenant_id"`
	TagPath      string  `json:"tag_path"`
	DesiredValue any     `json:"desired_value"`
	Operator     string  `json:"operator"`
	Status       string  `json:"status"` // SELECTED | OPERATED | CANCELLED | TIMED_OUT | INTERLOCK_DENIED
	Reason       string  `json:"reason,omitempty"`
	SelectedAt   int64   `json:"selected_at"`
	OperatedAt   *int64  `json:"operated_at,omitempty"`
}

// AlarmKPIs aggregates EEMUA 191 / ISA-18.2 alarm management performance metrics.
type AlarmKPIs struct {
	TotalAlarms      int     `json:"total_alarms"`
	UnackAlarms      int     `json:"unack_alarms"`
	AckedAlarms      int     `json:"acked_alarms"`
	ShelvedAlarms    int     `json:"shelved_alarms"`
	RtnUnackAlarms   int     `json:"rtn_unack_alarms"`
	NormalAlarms     int     `json:"normal_alarms"`
	MTTASeconds      float64 `json:"mtta_seconds"`     // Mean Time to Acknowledge
	AlarmRate10Min   float64 `json:"alarm_rate_10min"` // Alarms in trailing 10 min window
	InFlood          bool    `json:"in_flood"`         // Rate > 10 / 10 min
}

// ThingsBoard-Compatible SVG Symbol Models

type SymbolTag struct {
	Name      string `json:"name"`
	Role      string `json:"role"` // shape | text | status | icon
	ElementID string `json:"elementId"`
}

type SymbolProperty struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // color | string | number | boolean
	Default any    `json:"default"`
	Label   string `json:"label"`
}

type SymbolBehavior struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // value | action | widgetAction
	ValueType string `json:"valueType,omitempty"` // BOOL | DOUBLE | STRING | INT
	Label     string `json:"label"`
	Control   any    `json:"control,omitempty"`
}

type SymbolAnimation struct {
	Target string         `json:"target"`
	When   string         `json:"when"`
	Set    map[string]any `json:"set,omitempty"`
	Rotate map[string]any `json:"rotate,omitempty"`
}

// Symbol represents a ThingsBoard-compatible reusable industrial SVG symbol.
type Symbol struct {
	ID         ids.ID            `json:"id"`
	TenantID   *ids.ID           `json:"tenant_id,omitempty"`
	Name       string            `json:"name"`
	Category   string            `json:"category"` // Pumps | Valves | Tanks | Pipes | Meters | Motors
	Style      string            `json:"style"`    // standard | isa-101
	SVGContent string            `json:"svg_content"`
	Tags       []SymbolTag       `json:"tags"`
	Properties []SymbolProperty  `json:"properties"`
	Behaviors  []SymbolBehavior  `json:"behaviors"`
	Animations []SymbolAnimation `json:"animations"`
	Version    int               `json:"version"`
	CreatedAt  int64             `json:"created_at"`
}
