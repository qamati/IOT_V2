package edge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qamati/IOT_V2/pkg/ids"
)

func TestQueue_PriorityDrainAndAck(t *testing.T) {
	edgeID := ids.MustNew()
	q := NewQueue(edgeID, 100)
	ctx := context.Background()

	// Enqueue items across lanes
	_, _ = q.Enqueue(ctx, LaneL2, SyncItem{
		EntityID: ids.MustNew(),
		Type:     "LOG",
		Payload:  json.RawMessage(`{"msg":"diag log 1"}`),
	})
	_, _ = q.Enqueue(ctx, LaneL1, SyncItem{
		EntityID: ids.MustNew(),
		Type:     "TELEMETRY",
		Payload:  json.RawMessage(`[{"key":"temp","ts":1000,"dbl_v":21.5}]`),
	})
	_, _ = q.Enqueue(ctx, LaneL0, SyncItem{
		EntityID: ids.MustNew(),
		Type:     "ALARM",
		Payload:  json.RawMessage(`{"type":"HIGH_TEMP","severity":"CRITICAL"}`),
	})

	stats := q.Stats()
	if stats.TotalQueued != 3 || stats.LaneL0Queued != 1 || stats.LaneL1Queued != 1 || stats.LaneL2Queued != 1 {
		t.Fatalf("unexpected queue stats: %+v", stats)
	}

	// Drain batch: L0 must be in LaneL0, L1 in LaneL1, L2 in LaneL2
	batch, err := q.DrainBatch(ctx, 10)
	if err != nil || batch == nil {
		t.Fatalf("failed to drain batch: %v", err)
	}
	if len(batch.LaneL0) != 1 || batch.LaneL0[0].Type != "ALARM" {
		t.Errorf("expected L0 alarm, got %+v", batch.LaneL0)
	}
	if len(batch.LaneL1) != 1 || batch.LaneL1[0].Type != "TELEMETRY" {
		t.Errorf("expected L1 telemetry, got %+v", batch.LaneL1)
	}
	if len(batch.LaneL2) != 1 || batch.LaneL2[0].Type != "LOG" {
		t.Errorf("expected L2 log, got %+v", batch.LaneL2)
	}

	// Ack batch
	if err := q.Ack(batch.Sequence); err != nil {
		t.Fatalf("ack failed: %v", err)
	}

	// Verify empty after ack
	afterStats := q.Stats()
	if afterStats.TotalQueued != 0 || afterStats.AckedSeq != batch.Sequence {
		t.Fatalf("queue not empty after ack: %+v", afterStats)
	}
}

func TestQueue_CapacityPolicy(t *testing.T) {
	edgeID := ids.MustNew()
	q := NewQueue(edgeID, 2) // Cap at 2
	ctx := context.Background()

	// Fill with L2
	_, _ = q.Enqueue(ctx, LaneL2, SyncItem{Type: "LOG"})
	// Fill with L1
	_, _ = q.Enqueue(ctx, LaneL1, SyncItem{Type: "TELEMETRY"})

	// Enqueue high-priority L0: should evict L2 to make room
	_, err := q.Enqueue(ctx, LaneL0, SyncItem{Type: "ALARM"})
	if err != nil {
		t.Fatalf("failed to enqueue L0 under capacity limit: %v", err)
	}

	stats := q.Stats()
	if stats.LaneL0Queued != 1 || stats.LaneL2Queued != 0 {
		t.Errorf("expected L2 evicted to accommodate L0, stats: %+v", stats)
	}
}
