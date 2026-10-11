package bus

import (
	"context"
	"time"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// Standard event types across iotp
const (
	TypeTelemetryUp      = "POST_TELEMETRY_REQUEST"
	TypeAttributesUp     = "POST_ATTRIBUTES_REQUEST"
	TypeAttributesUpdate = "ATTRIBUTES_UPDATED"
	TypeDeviceConnected  = "DEVICE_CONNECTED"
	TypeDeviceDisconnect = "DEVICE_DISCONNECTED"
	TypeRpcRequest       = "RPC_REQUEST"
	TypeRpcResponse      = "RPC_RESPONSE"
	TypeAlarmCreated     = "ALARM_CREATED"
	TypeAlarmCleared     = "ALARM_CLEARED"
)

// Message is the standard event envelope circulating across the message bus.
type Message struct {
	ID             string            `json:"id"`
	TenantID       string            `json:"tenant_id"`
	OriginatorType string            `json:"originator_type"`
	OriginatorID   string            `json:"originator_id"`
	Type           string            `json:"type"`
	Timestamp      int64             `json:"timestamp"`
	Headers        map[string]string `json:"headers,omitempty"`
	Payload        []byte            `json:"payload"`
}

// PartitionKey returns the routing key for ordering guarantees (defaults to OriginatorID, then TenantID).
func (m *Message) PartitionKey() string {
	if m.OriginatorID != "" {
		return m.OriginatorID
	}
	if m.TenantID != "" {
		return m.TenantID
	}
	return m.ID
}

// NewMessage creates a message envelope with a generated UUIDv7 and current epoch ms timestamp.
func NewMessage(tenantID, origType, origID, msgType string, payload []byte) *Message {
	return &Message{
		ID:             ids.MustNew().String(),
		TenantID:       tenantID,
		OriginatorType: origType,
		OriginatorID:   origID,
		Type:           msgType,
		Timestamp:      time.Now().UnixMilli(),
		Headers:        make(map[string]string),
		Payload:        payload,
	}
}

// Handler handles a consumed message. Returning an error indicates processing failure (NACK/DLQ).
type Handler func(ctx context.Context, msg *Message) error

// Subscription represents an active message subscription.
type Subscription interface {
	Unsubscribe() error
}

// Bus defines the uniform interface for message brokers (Memory, NATS JetStream, Kafka).
type Bus interface {
	Publish(ctx context.Context, topic string, msg *Message) error
	Subscribe(ctx context.Context, topic string, group string, handler Handler) (Subscription, error)
	Close() error
}
