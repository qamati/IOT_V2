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
