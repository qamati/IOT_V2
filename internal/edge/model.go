package edge

import (
	"encoding/json"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// Edge Node statuses
const (
	StatusOnline  = "ONLINE"
	StatusOffline = "OFFLINE"
	StatusSyncing = "SYNCING"
)

// EdgeNode represents an autonomous edge computing instance.
type EdgeNode struct {
	ID               ids.ID   `json:"id"`
	TenantID         ids.ID   `json:"tenant_id"`
	Name             string   `json:"name"`
	EdgeKey          string   `json:"edge_key"`
	EdgeSecret       string   `json:"edge_secret,omitempty"`
	Status           string   `json:"status"` // ONLINE | OFFLINE | SYNCING
	FirmwareVersion  string   `json:"firmware_version"`
	HardwareModel    string   `json:"hardware_model"`
	LastSyncTs       int64    `json:"last_sync_ts"`
	BacklogCount     int64    `json:"backlog_count"`
	AssignedDevices  []ids.ID `json:"assigned_devices"`
	AssignedProfiles []ids.ID `json:"assigned_profiles"`
	Version          int      `json:"version"`
}

// SyncItem represents a single payload queued in an edge priority lane.
type SyncItem struct {
	EntityID  ids.ID          `json:"entity_id"`
	Timestamp int64           `json:"timestamp"`
	Type      string          `json:"type"` // TELEMETRY | ATTRIBUTE | ALARM
	Payload   json.RawMessage `json:"payload"`
}

// SyncBatch is uploaded by the edge node during store-and-forward catch-up.
type SyncBatch struct {
	EdgeID    ids.ID     `json:"edge_id"`
	Sequence  int64      `json:"sequence"`
	Timestamp int64      `json:"timestamp"`
	LaneL0    []SyncItem `json:"lane_l0"` // High-priority: Alarms & Control Events
	LaneL1    []SyncItem `json:"lane_l1"` // Medium-priority: Telemetry & State
	LaneL2    []SyncItem `json:"lane_l2"` // Low-priority: Bulk Historical Logs
}

// SyncAck confirms receipt of a sequence from the Edge Hub.
type SyncAck struct {
	EdgeID           ids.ID   `json:"edge_id"`
	AckSequence      int64    `json:"ack_sequence"`
	ServerTime       int64    `json:"server_time"`
	ProcessedItems   int      `json:"processed_items"`
	DownlinksPending int      `json:"downlinks_pending"`
	Downlinks        []any    `json:"downlinks,omitempty"`
}
