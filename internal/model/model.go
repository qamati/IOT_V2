package model

import (
	"encoding/json"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// Authority defines user role authorization levels.
type Authority string

const (
	AuthoritySysAdmin     Authority = "SYS_ADMIN"
	AuthorityTenantAdmin  Authority = "TENANT_ADMIN"
	AuthorityCustomerUser Authority = "CUSTOMER_USER"
)

// CredentialsType defines device authentication mechanisms.
type CredentialsType string

const (
	CredentialsTypeAccessToken     CredentialsType = "ACCESS_TOKEN"
	CredentialsTypeX509Certificate CredentialsType = "X509_CERTIFICATE"
	CredentialsTypeMqttBasic       CredentialsType = "MQTT_BASIC"
)

type TenantProfile struct {
	ID             ids.ID          `json:"id"`
	CreatedTime    int64           `json:"created_time"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	IsDefault      bool            `json:"is_default"`
	IsolationLevel int16           `json:"isolation_level"`
	ProfileData    json.RawMessage `json:"profile_data"`
	Version        int32           `json:"version"`
}

type Tenant struct {
	ID              ids.ID `json:"id"`
	CreatedTime     int64  `json:"created_time"`
	TenantProfileID ids.ID `json:"tenant_profile_id"`
	Title           string `json:"title"`
	Email           string `json:"email,omitempty"`
	Version         int32  `json:"version"`
}

type User struct {
	ID          ids.ID    `json:"id"`
	CreatedTime int64     `json:"created_time"`
	TenantID    *ids.ID   `json:"tenant_id,omitempty"`
	Authority   Authority `json:"authority"`
	Email       string    `json:"email"`
	FirstName   string    `json:"first_name,omitempty"`
	LastName    string    `json:"last_name,omitempty"`
	Version     int32     `json:"version"`
}

type DeviceProfile struct {
	ID            ids.ID          `json:"id"`
	CreatedTime   int64           `json:"created_time"`
	TenantID      ids.ID          `json:"tenant_id"`
	Name          string          `json:"name"`
	TransportType string          `json:"transport_type"`
	IsDefault     bool            `json:"is_default"`
	ProfileData   json.RawMessage `json:"profile_data"`
	Version       int32           `json:"version"`
}

type Device struct {
	ID              ids.ID   `json:"id"`
	CreatedTime     int64    `json:"created_time"`
	TenantID        ids.ID   `json:"tenant_id"`
	DeviceProfileID ids.ID   `json:"device_profile_id"`
	Name            string   `json:"name"`
	Label           string   `json:"label,omitempty"`
	IsGateway       bool     `json:"is_gateway"`
	Tags            []string `json:"tags,omitempty"`
	Version         int32    `json:"version"`
}

type DeviceCredentials struct {
	ID               ids.ID          `json:"id"`
	CreatedTime      int64           `json:"created_time"`
	TenantID         ids.ID          `json:"tenant_id"`
	DeviceID         ids.ID          `json:"device_id"`
	CredentialsType  CredentialsType `json:"credentials_type"`
	LookupID         []byte          `json:"-"`
	CredentialsValue string          `json:"credentials_value,omitempty"`
}

// TelemetryDataPoint represents a single key-value reading at a timestamp.
type TelemetryDataPoint struct {
	Key       string  `json:"key"`
	Timestamp int64   `json:"ts"`
	BoolV     *bool   `json:"bool_v,omitempty"`
	StrV      *string `json:"str_v,omitempty"`
	LongV     *int64  `json:"long_v,omitempty"`
	DblV      *float64`json:"dbl_v,omitempty"`
	JsonV     []byte  `json:"json_v,omitempty"`
}

// Value returns the untyped representation of the value.
func (p *TelemetryDataPoint) Value() any {
	if p.BoolV != nil {
		return *p.BoolV
	}
	if p.StrV != nil {
		return *p.StrV
	}
	if p.LongV != nil {
		return *p.LongV
	}
	if p.DblV != nil {
		return *p.DblV
	}
	if p.JsonV != nil {
		return json.RawMessage(p.JsonV)
	}
	return nil
}

// AttributeScope defines the visibility and ownership of entity attributes.
type AttributeScope string

const (
	ScopeClient AttributeScope = "CLIENT_SCOPE"
	ScopeServer AttributeScope = "SERVER_SCOPE"
	ScopeShared AttributeScope = "SHARED_SCOPE"
)

// AttributeKv represents an attribute key-value pair.
type AttributeKv struct {
	EntityID   ids.ID         `json:"entity_id"`
	Scope      AttributeScope `json:"scope"`
	Key        string         `json:"key"`
	LastUpdate int64          `json:"last_update_ts"`
	Val        any            `json:"value"`
}

// AlarmSeverity defines alarm priority levels.
type AlarmSeverity string

const (
	SeverityCritical      AlarmSeverity = "CRITICAL"
	SeverityMajor         AlarmSeverity = "MAJOR"
	SeverityMinor         AlarmSeverity = "MINOR"
	SeverityWarning       AlarmSeverity = "WARNING"
	SeverityIndeterminate AlarmSeverity = "INDETERMINATE"
)

// AlarmStatus defines alarm acknowledgment and clearance states.
type AlarmStatus string

const (
	AlarmActiveUnack  AlarmStatus = "ACTIVE_UNACK"
	AlarmActiveAck    AlarmStatus = "ACTIVE_ACK"
	AlarmClearedUnack AlarmStatus = "CLEARED_UNACK"
	AlarmClearedAck   AlarmStatus = "CLEARED_ACK"
)

// Alarm represents an operational condition or alert on an entity.
type Alarm struct {
	ID           ids.ID          `json:"id"`
	CreatedTime  int64           `json:"created_time"`
	TenantID     ids.ID          `json:"tenant_id"`
	OriginatorID ids.ID          `json:"originator_id"`
	Type         string          `json:"type"`
	Severity     AlarmSeverity   `json:"severity"`
	Status       AlarmStatus     `json:"status"`
	StartTs      int64           `json:"start_ts"`
	EndTs        int64           `json:"end_ts,omitempty"`
	AckTs        int64           `json:"ack_ts,omitempty"`
	ClearTs      int64           `json:"clear_ts,omitempty"`
	Details      json.RawMessage `json:"details,omitempty"`
}

// DeviceActivity tracks online/offline status and heartbeat timestamps.
type DeviceActivity struct {
	DeviceID         ids.ID `json:"device_id"`
	LastActivityTime int64  `json:"last_activity_time"`
	Active           bool   `json:"active"`
}

// Dashboard represents an analytical visualization canvas.
type Dashboard struct {
	ID            ids.ID          `json:"id"`
	CreatedTime   int64           `json:"created_time"`
	TenantID      ids.ID          `json:"tenant_id"`
	Title         string          `json:"title"`
	Image         string          `json:"image,omitempty"`
	Configuration json.RawMessage `json:"configuration"`
}

// RpcStatus tracks lifecycle of a device downlink command.
type RpcStatus string

const (
	RpcStatusQueued     RpcStatus = "QUEUED"
	RpcStatusDelivered  RpcStatus = "DELIVERED"
	RpcStatusSuccessful RpcStatus = "SUCCESSFUL"
	RpcStatusTimeout    RpcStatus = "TIMEOUT"
	RpcStatusFailed     RpcStatus = "FAILED"
)

// RpcCommand represents a server-to-device remote procedure call.
type RpcCommand struct {
	ID           ids.ID          `json:"id"`
	TenantID     ids.ID          `json:"tenant_id"`
	DeviceID     ids.ID          `json:"device_id"`
	Method       string          `json:"method"`
	Params       json.RawMessage `json:"params"`
	TwoWay       bool            `json:"two_way"`
	Status       RpcStatus       `json:"status"`
	CreatedTime  int64           `json:"created_time"`
	ResponseTime int64           `json:"response_time,omitempty"`
	Response     json.RawMessage `json:"response,omitempty"`
}
