package edge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Service manages Edge nodes and processes store-and-forward edge sync batches.
type Service interface {
	CreateEdge(ctx context.Context, e *EdgeNode) error
	GetEdge(ctx context.Context, tenantID, edgeID ids.ID) (*EdgeNode, error)
	ListEdges(ctx context.Context, tenantID ids.ID) ([]*EdgeNode, error)
	DeleteEdge(ctx context.Context, tenantID, edgeID ids.ID) error
	ProcessSync(ctx context.Context, batch *SyncBatch) (*SyncAck, error)
}

type memoryHub struct {
	mu           sync.RWMutex
	edges        map[ids.ID]*EdgeNode
	lastSequence map[ids.ID]int64

	telemetry telemetry.Store
	alarms    alarm.Service
}

func NewMemoryHub(ts telemetry.Store, al alarm.Service) Service {
	hub := &memoryHub{
		edges:        make(map[ids.ID]*EdgeNode),
		lastSequence: make(map[ids.ID]int64),
		telemetry:    ts,
		alarms:       al,
	}

	hub.seedDemoEdge()
	return hub
}

func (h *memoryHub) seedDemoEdge() {
	edgeID := ids.MustNew()
	h.edges[edgeID] = &EdgeNode{
		ID:               edgeID,
		TenantID:         ids.Nil, // system / demo
		Name:             "edge-gateway-facility-01",
		EdgeKey:          "EDGE_FACILITY_KEY_01",
		EdgeSecret:       "sec_edge_9876543210",
		Status:           StatusOnline,
		FirmwareVersion:  "v1.4.2-edge",
		HardwareModel:    "Advantech UNO-2271G",
		LastSyncTs:       time.Now().UnixMilli() - 15000,
		BacklogCount:     0,
		AssignedDevices:  []ids.ID{},
		AssignedProfiles: []ids.ID{},
		Version:          1,
	}
	h.lastSequence[edgeID] = 1042
}

func (h *memoryHub) CreateEdge(ctx context.Context, e *EdgeNode) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if e.ID == ids.Nil {
		e.ID = ids.MustNew()
	}
	if e.EdgeKey == "" {
		e.EdgeKey = fmt.Sprintf("EDGE_%s", e.ID.String()[:8])
	}
	if e.Status == "" {
		e.Status = StatusOnline
	}
	if e.AssignedDevices == nil {
		e.AssignedDevices = []ids.ID{}
	}
	if e.AssignedProfiles == nil {
		e.AssignedProfiles = []ids.ID{}
	}
	e.Version = 1

	h.edges[e.ID] = e
	h.lastSequence[e.ID] = 0
	return nil
}

func (h *memoryHub) GetEdge(ctx context.Context, tenantID, edgeID ids.ID) (*EdgeNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	e, ok := h.edges[edgeID]
	if !ok || (e.TenantID != ids.Nil && e.TenantID != tenantID) {
		return nil, errs.NotFound("edge node not found")
	}
	return e, nil
}

func (h *memoryHub) ListEdges(ctx context.Context, tenantID ids.ID) ([]*EdgeNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*EdgeNode
	for _, e := range h.edges {
		if e.TenantID == ids.Nil || e.TenantID == tenantID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (h *memoryHub) DeleteEdge(ctx context.Context, tenantID, edgeID ids.ID) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	e, ok := h.edges[edgeID]
	if !ok || (e.TenantID != ids.Nil && e.TenantID != tenantID) {
		return errs.NotFound("edge node not found")
	}
	delete(h.edges, edgeID)
	delete(h.lastSequence, edgeID)
	return nil
}

func (h *memoryHub) ProcessSync(ctx context.Context, batch *SyncBatch) (*SyncAck, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	edge, ok := h.edges[batch.EdgeID]
	if !ok {
		return nil, errs.NotFound("edge node not found")
	}

	// Sequence check (idempotency / deduplication)
	lastSeq := h.lastSequence[batch.EdgeID]
	if batch.Sequence <= lastSeq && batch.Sequence != 0 {
		// Already processed, return idempotently
		return &SyncAck{
			EdgeID:         batch.EdgeID,
			AckSequence:    lastSeq,
			ServerTime:     time.Now().UnixMilli(),
			ProcessedItems: 0,
		}, nil
	}

	processedCount := 0

	// Process L0: Alarms
	for _, item := range batch.LaneL0 {
		if item.Type == "ALARM" && h.alarms != nil {
			var al model.Alarm
			if err := json.Unmarshal(item.Payload, &al); err == nil {
				al.TenantID = edge.TenantID
				al.OriginatorID = item.EntityID
				_, _ = h.alarms.CreateOrUpdateAlarm(ctx, &al)
				processedCount++
			}
		}
	}

	// Process L1 & L2: Telemetry
	var allTelemetry []*model.TelemetryDataPoint
	for _, item := range append(batch.LaneL1, batch.LaneL2...) {
		if item.Type == "TELEMETRY" && h.telemetry != nil {
			var points []*model.TelemetryDataPoint
			if err := json.Unmarshal(item.Payload, &points); err == nil {
				allTelemetry = append(allTelemetry, points...)
				_ = h.telemetry.Save(ctx, item.EntityID, points)
				processedCount += len(points)
			}
		}
	}

	// Update edge state
	edge.LastSyncTs = time.Now().UnixMilli()
	edge.Status = StatusOnline
	edge.BacklogCount = 0
	h.lastSequence[batch.EdgeID] = batch.Sequence

	return &SyncAck{
		EdgeID:           batch.EdgeID,
		AckSequence:      batch.Sequence,
		ServerTime:       time.Now().UnixMilli(),
		ProcessedItems:   processedCount,
		DownlinksPending: 0,
	}, nil
}
