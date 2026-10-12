package model

import (
	"encoding/json"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// NotificationTargetType specifies the delivery destination mechanism.
type NotificationTargetType string

const (
	TargetPlatformUsers NotificationTargetType = "PLATFORM_USERS"
	TargetEmail         NotificationTargetType = "EMAIL"
	TargetSlack         NotificationTargetType = "SLACK"
	TargetWebhook       NotificationTargetType = "WEBHOOK"
	TargetSMS           NotificationTargetType = "SMS"
)

// NotificationTarget defines a recipient group or endpoint configuration.
type NotificationTarget struct {
	ID          ids.ID                 `json:"id"`
	CreatedTime int64                  `json:"created_time"`
	TenantID    ids.ID                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	Type        NotificationTargetType `json:"type"`
	Config      json.RawMessage        `json:"config"` // e.g. emails: [], slack_webhook_url: "", users: []
}

// NotificationTemplate defines formatting and variables for notification channels.
type NotificationTemplate struct {
	ID          ids.ID          `json:"id"`
	CreatedTime int64           `json:"created_time"`
	TenantID    ids.ID          `json:"tenant_id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"` // ALARM, DEVICE_ACTIVITY, GENERAL
	Config      json.RawMessage `json:"config"` // subject, body_inbox, body_email, body_slack
}

// EscalationStep defines a timed delivery tier to recipient targets.
type EscalationStep struct {
	Step         int      `json:"step"`
	DelaySeconds int      `json:"delay_seconds"`
	TargetIDs    []ids.ID `json:"target_ids"`
}

// NotificationRule maps event triggers to templates and escalation recipient tiers.
type NotificationRule struct {
	ID               ids.ID           `json:"id"`
	CreatedTime      int64            `json:"created_time"`
	TenantID         ids.ID           `json:"tenant_id"`
	Name             string           `json:"name"`
	Enabled          bool             `json:"enabled"`
	TemplateID       ids.ID           `json:"template_id"`
	TriggerType      string           `json:"trigger_type"` // ALARM, ALARM_CLEAR, ALARM_ACK, DEVICE_OFFLINE
	TriggerConfig    json.RawMessage  `json:"trigger_config"` // e.g. severities: ["CRITICAL", "MAJOR"]
	RecipientsConfig []EscalationStep `json:"recipients_config"`
	Throttle         json.RawMessage  `json:"throttle,omitempty"`
}

// NotificationRequest records a batch or scheduled notification delivery execution.
type NotificationRequest struct {
	ID          ids.ID          `json:"id"`
	CreatedTime int64           `json:"created_time"`
	TenantID    ids.ID          `json:"tenant_id"`
	RuleID      *ids.ID         `json:"rule_id,omitempty"`
	TemplateID  *ids.ID         `json:"template_id,omitempty"`
	Targets     []ids.ID        `json:"targets"`
	Info        json.RawMessage `json:"info,omitempty"`
	Status      string          `json:"status"`
	Stats       json.RawMessage `json:"stats,omitempty"`
}

// NotificationStatus tracks in-app inbox reading status.
type NotificationStatus string

const (
	NotificationUnread NotificationStatus = "UNREAD"
	NotificationRead   NotificationStatus = "READ"
)

// Notification represents an in-app inbox message item.
type Notification struct {
	ID          ids.ID             `json:"id"`
	CreatedTime int64              `json:"created_time"`
	TenantID    ids.ID             `json:"tenant_id"`
	RecipientID ids.ID             `json:"recipient_id"`
	RequestID   *ids.ID            `json:"request_id,omitempty"`
	Type        string             `json:"type"`
	Subject     string             `json:"subject"`
	Body        string             `json:"body"`
	Info        json.RawMessage    `json:"info,omitempty"`
	Status      NotificationStatus `json:"status"`
}

// NotificationDelivery records a transmission attempt to an external channel.
type NotificationDelivery struct {
	ID          ids.ID `json:"id"`
	CreatedTime int64  `json:"created_time"`
	TenantID    ids.ID `json:"tenant_id"`
	RequestID   ids.ID `json:"request_id"`
	Channel     string `json:"channel"` // INBOX, EMAIL, SLACK, WEBHOOK
	Recipient   string `json:"recipient"`
	Status      string `json:"status"` // SENT, FAILED
	LastError   string `json:"last_error,omitempty"`
}

// NotificationSendRequest allows triggering manual or test notifications.
type NotificationSendRequest struct {
	Targets []ids.ID        `json:"targets"`
	Subject string          `json:"subject"`
	Body    string          `json:"body"`
	Info    json.RawMessage `json:"info,omitempty"`
}
