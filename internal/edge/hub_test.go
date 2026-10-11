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
