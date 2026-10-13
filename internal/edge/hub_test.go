package edge

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestEdgeHub_CreateAndSync(t *testing.T) {
	hub := NewMemoryHub(nil, nil)
	ctx := context.Background()

	edgeID := ids.MustNew()
	tenantID := ids.MustNew()
	node := &EdgeNode{
		ID:              edgeID,
		TenantID:        tenantID,
		Name:            "edge-node-line-4",
		HardwareModel:   "Raspberry Pi 4",
		FirmwareVersion: "v1.2.0",
	}

	if err := hub.CreateEdge(ctx, node); err != nil {
		t.Fatalf("failed to create edge: %v", err)
	}

	// Verify retrieval
	retrieved, err := hub.GetEdge(ctx, tenantID, edgeID)
	if err != nil || retrieved.Name != "edge-node-line-4" {
		t.Fatalf("failed to get edge: %v", err)
	}

	// Simulate sync batch
	batch := &SyncBatch{
		EdgeID:    edgeID,
		Sequence:  1,
		Timestamp: time.Now().UnixMilli(),
		LaneL1: []SyncItem{
			{
				EntityID:  ids.MustNew(),
				Timestamp: time.Now().UnixMilli(),
				Type:      "TELEMETRY",
				Payload:   json.RawMessage(`[{"key":"flow_rate","ts":1700000000,"dbl_v":14.8}]`),
			},
		},
	}

	ack, err := hub.ProcessSync(ctx, batch)
	if err != nil {
		t.Fatalf("sync error: %v", err)
	}
	if ack.AckSequence != 1 {
		t.Errorf("expected ack sequence 1, got %d", ack.AckSequence)
	}

	// Replay duplicate batch (must be acknowledged idempotently with 0 processed)
	ackDup, err := hub.ProcessSync(ctx, batch)
	if err != nil {
		t.Fatalf("replay sync error: %v", err)
	}
	if ackDup.ProcessedItems != 0 {
		t.Errorf("expected 0 processed items for duplicate sequence, got %d", ackDup.ProcessedItems)
	}
}

func TestEdgeHub_DeviceAssignments(t *testing.T) {
	hub := NewMemoryHub(nil, nil)
	ctx := context.Background()

	edgeID := ids.MustNew()
	tenantID := ids.MustNew()
	node := &EdgeNode{
		ID:       edgeID,
		TenantID: tenantID,
		Name:     "gateway-plc-station",
	}
	if err := hub.CreateEdge(ctx, node); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	dev1 := ids.MustNew()
	dev2 := ids.MustNew()

	// Assign devices
	if err := hub.AssignDevices(ctx, edgeID, []ids.ID{dev1, dev2}); err != nil {
		t.Fatalf("assign devices: %v", err)
	}

	assigned, err := hub.GetAssignedDevices(ctx, edgeID)
	if err != nil {
		t.Fatalf("get assigned devices: %v", err)
	}
	if len(assigned) != 2 {
		t.Fatalf("expected 2 assigned devices, got %d", len(assigned))
	}

	// Unassign dev1
	if err := hub.UnassignDevice(ctx, edgeID, dev1); err != nil {
		t.Fatalf("unassign device: %v", err)
	}

	assignedAfter, err := hub.GetAssignedDevices(ctx, edgeID)
	if err != nil {
		t.Fatalf("get assigned after: %v", err)
	}
	if len(assignedAfter) != 1 || assignedAfter[0] != dev2 {
		t.Fatalf("expected only dev2 assigned, got %v", assignedAfter)
	}
}

func TestEdgeHub_RemoteCommands(t *testing.T) {
	hub := NewMemoryHub(nil, nil)
	ctx := context.Background()

	edgeID := ids.MustNew()
	tenantID := ids.MustNew()
	node := &EdgeNode{
		ID:       edgeID,
		TenantID: tenantID,
		Name:     "hvac-edge-controller",
	}
	_ = hub.CreateEdge(ctx, node)

	// Test REBOOT command
	resp, err := hub.ExecuteRemoteCommand(ctx, tenantID, edgeID, &RemoteCommandReq{Command: "REBOOT"})
	if err != nil || !resp.Success {
		t.Fatalf("failed reboot command: %v", err)
	}

	// Test FORCE_SYNC command
	respSync, err := hub.ExecuteRemoteCommand(ctx, tenantID, edgeID, &RemoteCommandReq{Command: "FORCE_SYNC"})
	if err != nil || !respSync.Success {
		t.Fatalf("failed force sync command: %v", err)
	}

	// Test unknown command
	_, errInvalid := hub.ExecuteRemoteCommand(ctx, tenantID, edgeID, &RemoteCommandReq{Command: "UNKNOWN_ACTION"})
	if errInvalid == nil {
		t.Fatalf("expected error for invalid command")
	}
}

func TestEdgeHub_ConfigurationAndHeartbeat(t *testing.T) {
	hub := NewMemoryHub(nil, nil)
	ctx := context.Background()

	edgeID := ids.MustNew()
	tenantID := ids.MustNew()
	key := "EDGE_TEST_KEY_44"
	node := &EdgeNode{
		ID:       edgeID,
		TenantID: tenantID,
		Name:     "solar-inverter-edge",
		EdgeKey:  key,
	}
	_ = hub.CreateEdge(ctx, node)

	// Test heartbeat
	if err := hub.Heartbeat(ctx, key); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	// Test get configuration
	cfg, err := hub.GetConfiguration(ctx, key)
	if err != nil {
		t.Fatalf("get configuration failed: %v", err)
	}
	if cfg.EdgeID != edgeID || cfg.Name != "solar-inverter-edge" {
		t.Fatalf("config mismatch: %+v", cfg)
	}
}

