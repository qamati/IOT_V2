package rule

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Standard message types conforming to ThingsBoard and IoT Platform specs.
const (
	TypePostTelemetry    = "POST_TELEMETRY_REQUEST"
	TypePostAttributes   = "POST_ATTRIBUTES_REQUEST"
	TypeAttributesUpdated = "ATTRIBUTES_UPDATED"
	TypeActivityEvent    = "ACTIVITY_EVENT"
	TypeInactivityEvent  = "INACTIVITY_EVENT"
	TypeConnectEvent     = "CONNECT_EVENT"
	TypeDisconnectEvent  = "DISCONNECT_EVENT"
	TypeAlarm            = "ALARM"
	TypeAlarmAck         = "ALARM_ACK"
	TypeAlarmClear       = "ALARM_CLEAR"
	TypeRpcRequest       = "TO_SERVER_RPC_REQUEST"
)

// Standard relation labels emitted by rule nodes.
const (
	RelationSuccess         = "Success"
	RelationFailure         = "Failure"
	RelationTrue            = "True"
	RelationFalse           = "False"
	RelationPostTelemetry   = "Post telemetry"
	RelationPostAttributes  = "Post attributes"
	RelationActivityEvent   = "Activity Event"
	RelationInactivityEvent = "Inactivity Event"
	RelationOther           = "Other"
	RelationCreated         = "Created"
	RelationUpdated         = "Updated"
	RelationCleared         = "Cleared"
)

// Message is the universal unit of execution that traverses a rule chain graph.
type Message struct {
	ID         ids.ID            `json:"id"`
	TenantID   ids.ID            `json:"tenantId"`
	Ts         int64             `json:"ts"`
	Type       string            `json:"type"`
	Originator model.EntityRef   `json:"originator"`
	CustomerID *ids.ID           `json:"customerId,omitempty"`
	Metadata   map[string]string `json:"metadata"`
	DataType   string            `json:"dataType"` // "JSON", "TEXT", "BINARY"
	Data       []byte            `json:"data"`
	Queue      string            `json:"queue,omitempty"`
	ChainID    ids.ID            `json:"chainId,omitempty"`
	Hops       uint8             `json:"hops"`
}

// Clone returns a shallow copy of the message with a newly allocated metadata map.
func (m *Message) Clone() *Message {
	metaCopy := make(map[string]string, len(m.Metadata))
	for k, v := range m.Metadata {
		metaCopy[k] = v
	}
	cp := *m
	cp.Metadata = metaCopy
	return &cp
}

// Services bundles platform services accessible to rule nodes during execution.
type Services struct {
	Telemetry  telemetry.Store
	Attributes telemetry.AttributeStore
	Alarms     alarm.Service
	DevState   devstate.Tracker
	HTTPClient *http.Client
}

// Env provides the initialization context to a rule node.
type Env interface {
	Services() *Services
	Logger() *slog.Logger
	TenantID() ids.ID
}

// NodeCtx provides runtime execution capabilities to an executing node.
type NodeCtx interface {
	TellNext(msg *Message, relations ...string)
	TellSuccess(msg *Message)
	TellFailure(msg *Message, err error)
	Services() *Services
	Log() *slog.Logger
	TenantID() ids.ID
}

// Node is the interface implemented by all rule engine nodes in the catalog.
type Node interface {
	Init(cfg json.RawMessage, env Env) error
	OnMsg(ctx NodeCtx, msg *Message)
	Close()
}

// Descriptor declares the metadata, schemas, and ports for UI rendering and validation.
type Descriptor struct {
	Type         string          `json:"type"`
	Name         string          `json:"name"`
	Category     string          `json:"category"` // FILTER, ENRICHMENT, TRANSFORMATION, ACTION, EXTERNAL, FLOW
	Relations    []string        `json:"relations"`
	ConfigSchema json.RawMessage `json:"configSchema"`
	UISchema     json.RawMessage `json:"uiSchema,omitempty"`
	Version      int             `json:"version"`
	Docs         string          `json:"docs,omitempty"`
}

// NodeTraceRecord records the debug trace of a single node invocation.
type NodeTraceRecord struct {
	NodeID       string            `json:"nodeId"`
	NodeType     string            `json:"nodeType"`
	InMessage    *Message          `json:"inMessage"`
	OutMessage   *Message          `json:"outMessage,omitempty"`
	Relations    []string          `json:"relations"`
	Error        string            `json:"error,omitempty"`
	DurationMs   float64           `json:"durationMs"`
	Timestamp    int64             `json:"timestamp"`
}

// ExecutionTrace captures an end-to-end execution path through a rule chain.
type ExecutionTrace struct {
	MessageID   ids.ID             `json:"messageId"`
	ChainID     ids.ID             `json:"chainId"`
	Success     bool               `json:"success"`
	Records     []*NodeTraceRecord `json:"records"`
	TotalTimeMs float64            `json:"totalTimeMs"`
	Error       string             `json:"error,omitempty"`
}
