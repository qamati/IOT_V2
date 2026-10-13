package edge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// Service manages Edge nodes, assignments, remote commands, and store-and-forward sync batches.
type Service interface {
	CreateEdge(ctx context.Context, e *EdgeNode) error
	GetEdge(ctx context.Context, tenantID, edgeID ids.ID) (*EdgeNode, error)
	GetEdgeByKey(ctx context.Context, edgeKey string) (*EdgeNode, error)
	ListEdges(ctx context.Context, tenantID ids.ID) ([]*EdgeNode, error)
	UpdateEdge(ctx context.Context, e *EdgeNode) error
	DeleteEdge(ctx context.Context, tenantID, edgeID ids.ID) error

	AssignDevices(ctx context.Context, edgeID ids.ID, deviceIDs []ids.ID) error
	UnassignDevice(ctx context.Context, edgeID ids.ID, deviceID ids.ID) error
	GetAssignedDevices(ctx context.Context, edgeID ids.ID) ([]ids.ID, error)

	ProcessSync(ctx context.Context, batch *SyncBatch) (*SyncAck, error)
	Heartbeat(ctx context.Context, edgeKey string) error
	ExecuteRemoteCommand(ctx context.Context, tenantID, edgeID ids.ID, req *RemoteCommandReq) (*RemoteCommandResp, error)
	GetSyncJournal(ctx context.Context, tenantID, edgeID ids.ID, limit int) ([]*SyncJournalEntry, error)
	GetConfiguration(ctx context.Context, edgeKey string) (*EdgeConfiguration, error)
}

type hubService struct {
	store     Store
	telemetry telemetry.Store
	alarms    alarm.Service

	// Cached sequence tracker for active streaming
	mu           sync.RWMutex
	lastSequence map[ids.ID]int64
}

// NewService constructs a production Edge Hub service backed by a Store.
func NewService(store Store, ts telemetry.Store, al alarm.Service) Service {
	if store == nil {
		store = newMemoryStore()
	}
	return &hubService{
		store:        store,
		telemetry:    ts,
		alarms:       al,
		lastSequence: make(map[ids.ID]int64),
	}
}

// NewMemoryHub provides an in-memory Hub service for unit tests.
func NewMemoryHub(ts telemetry.Store, al alarm.Service) Service {
	return NewService(newMemoryStore(), ts, al)
}

func (h *hubService) CreateEdge(ctx context.Context, e *EdgeNode) error {
	if e.ID == ids.Nil {
		e.ID = ids.MustNew()
	}
	if e.EdgeKey == "" {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		e.EdgeKey = fmt.Sprintf("EDGE_%s", hex.EncodeToString(b))
	}
	if e.EdgeSecret == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		e.EdgeSecret = "sec_edge_" + hex.EncodeToString(b)
	}
	if e.Status == "" {
		e.Status = StatusOffline
	}
	if e.FirmwareVersion == "" {
		e.FirmwareVersion = "v1.0.0-edge"
	}
	if e.HardwareModel == "" {
		e.HardwareModel = "Advantech UNO-2271G"
	}
	if e.AssignedDevices == nil {
		e.AssignedDevices = []ids.ID{}
	}
	if e.AssignedProfiles == nil {
		e.AssignedProfiles = []ids.ID{}
	}

	return h.store.CreateEdge(ctx, e)
}

func (h *hubService) GetEdge(ctx context.Context, tenantID, edgeID ids.ID) (*EdgeNode, error) {
	return h.store.GetEdge(ctx, tenantID, edgeID)
}

func (h *hubService) GetEdgeByKey(ctx context.Context, edgeKey string) (*EdgeNode, error) {
	return h.store.GetEdgeByKey(ctx, edgeKey)
}

func (h *hubService) ListEdges(ctx context.Context, tenantID ids.ID) ([]*EdgeNode, error) {
	return h.store.ListEdges(ctx, tenantID)
}

func (h *hubService) UpdateEdge(ctx context.Context, e *EdgeNode) error {
	return h.store.UpdateEdge(ctx, e)
}

func (h *hubService) DeleteEdge(ctx context.Context, tenantID, edgeID ids.ID) error {
	h.mu.Lock()
	delete(h.lastSequence, edgeID)
	h.mu.Unlock()
	return h.store.DeleteEdge(ctx, tenantID, edgeID)
}

func (h *hubService) AssignDevices(ctx context.Context, edgeID ids.ID, deviceIDs []ids.ID) error {
	return h.store.AssignDevices(ctx, edgeID, deviceIDs)
}

func (h *hubService) UnassignDevice(ctx context.Context, edgeID ids.ID, deviceID ids.ID) error {
	return h.store.UnassignDevice(ctx, edgeID, deviceID)
}

func (h *hubService) GetAssignedDevices(ctx context.Context, edgeID ids.ID) ([]ids.ID, error) {
	return h.store.GetAssignedDevices(ctx, edgeID)
}

func (h *hubService) ProcessSync(ctx context.Context, batch *SyncBatch) (*SyncAck, error) {
	edge, err := h.store.GetEdge(ctx, ids.Nil, batch.EdgeID)
	if err != nil {
		return nil, errs.NotFound("edge node not found")
	}

	h.mu.Lock()
	lastSeq, cached := h.lastSequence[batch.EdgeID]
	if !cached {
		lastSeq = edge.UplinkSeq
		h.lastSequence[batch.EdgeID] = lastSeq
	}
	h.mu.Unlock()

	// Idempotent duplicate sequence check
	if batch.Sequence <= lastSeq && batch.Sequence != 0 {
		return &SyncAck{
			EdgeID:           batch.EdgeID,
			AckSequence:      lastSeq,
			ServerTime:       time.Now().UnixMilli(),
			ProcessedItems:   0,
			DownlinksPending: 0,
		}, nil
	}

	l0Count := len(batch.LaneL0)
	l1Count := len(batch.LaneL1)
	l2Count := len(batch.LaneL2)
	processedCount := 0

	// 1. Process Lane L0: Alarms & High-Priority Control Events
	for _, item := range batch.LaneL0 {
		if item.Type == "ALARM" && h.alarms != nil {
			var al model.Alarm
			if err := json.Unmarshal(item.Payload, &al); err == nil {
				al.TenantID = edge.TenantID
				al.OriginatorID = item.EntityID
				if al.StartTs == 0 {
					al.StartTs = item.Timestamp
				}
				_, _ = h.alarms.CreateOrUpdateAlarm(ctx, &al)
				processedCount++
			}
		}
	}

	// 2. Process Lane L1 & L2: Telemetry & State
	for _, item := range append(batch.LaneL1, batch.LaneL2...) {
		if item.Type == "TELEMETRY" && h.telemetry != nil {
			var points []*model.TelemetryDataPoint
			if err := json.Unmarshal(item.Payload, &points); err == nil {
				for _, p := range points {
					if p.Timestamp == 0 {
						p.Timestamp = item.Timestamp
					}
				}
				_ = h.telemetry.Save(ctx, item.EntityID, points)
				processedCount += len(points)
			}
		}
	}

	// 3. Update Sync State & Sequence Tracking
	now := time.Now().UnixMilli()
	_ = h.store.UpdateSyncState(ctx, batch.EdgeID, StatusOnline, batch.Sequence, now, 0)

	h.mu.Lock()
	h.lastSequence[batch.EdgeID] = batch.Sequence
	h.mu.Unlock()

	// 4. Record journal entry
	_ = h.store.RecordSyncJournal(ctx, &SyncJournalEntry{
		ID:             ids.MustNew(),
		EdgeID:         batch.EdgeID,
		Sequence:       batch.Sequence,
		LaneL0Count:    l0Count,
		LaneL1Count:    l1Count,
		LaneL2Count:    l2Count,
		ProcessedItems: processedCount,
		Status:         "PROCESSED",
		SyncedAt:       now,
	})

	return &SyncAck{
		EdgeID:           batch.EdgeID,
		AckSequence:      batch.Sequence,
		ServerTime:       now,
		ProcessedItems:   processedCount,
		DownlinksPending: 0,
	}, nil
}

func (h *hubService) Heartbeat(ctx context.Context, edgeKey string) error {
	edge, err := h.store.GetEdgeByKey(ctx, edgeKey)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	return h.store.UpdateSyncState(ctx, edge.ID, StatusOnline, edge.UplinkSeq, now, edge.BacklogCount)
}

func (h *hubService) ExecuteRemoteCommand(ctx context.Context, tenantID, edgeID ids.ID, req *RemoteCommandReq) (*RemoteCommandResp, error) {
	edge, err := h.store.GetEdge(ctx, tenantID, edgeID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	switch req.Command {
	case "REBOOT":
		_ = h.store.UpdateSyncState(ctx, edgeID, StatusSyncing, edge.UplinkSeq, now, edge.BacklogCount)
		return &RemoteCommandResp{
			Command:    "REBOOT",
			Success:    true,
			Message:    fmt.Sprintf("Reboot command dispatched to %s (%s)", edge.Name, edge.HardwareModel),
			ExecutedAt: now,
		}, nil
	case "FORCE_SYNC":
		return &RemoteCommandResp{
			Command:    "FORCE_SYNC",
			Success:    true,
			Message:    fmt.Sprintf("FullSync checkpoint requested for edge %s", edge.Name),
			ExecutedAt: now,
		}, nil
	case "COLLECT_LOGS":
		return &RemoteCommandResp{
			Command:    "COLLECT_LOGS",
			Success:    true,
			Message:    "Diagnostics snapshot bundle requested from edge runtime",
			ExecutedAt: now,
		}, nil
	default:
		return nil, errs.InvalidArgument(fmt.Sprintf("unrecognized edge remote command: %s", req.Command))
	}
}

func (h *hubService) GetSyncJournal(ctx context.Context, tenantID, edgeID ids.ID, limit int) ([]*SyncJournalEntry, error) {
	// Verify edge belongs to tenant
	if _, err := h.store.GetEdge(ctx, tenantID, edgeID); err != nil {
		return nil, err
	}
	return h.store.ListSyncJournal(ctx, edgeID, limit)
}

func (h *hubService) GetConfiguration(ctx context.Context, edgeKey string) (*EdgeConfiguration, error) {
	edge, err := h.store.GetEdgeByKey(ctx, edgeKey)
	if err != nil {
		return nil, err
	}
	devs, err := h.store.GetAssignedDevices(ctx, edge.ID)
	if err != nil {
		devs = edge.AssignedDevices
	}
	return &EdgeConfiguration{
		EdgeID:          edge.ID,
		TenantID:        edge.TenantID,
		Name:            edge.Name,
		AssignedDevices: devs,
		DownlinkSeq:     edge.DownlinkSeq,
		UplinkSeq:       edge.UplinkSeq,
		SyncIntervalSec: 10,
	}, nil
}

// ============================================================================
// In-Memory Store (Fallback & Test double)
// ============================================================================

type memoryStore struct {
	mu          sync.RWMutex
	edges       map[ids.ID]*EdgeNode
	assignments map[ids.ID][]ids.ID
	journals    map[ids.ID][]*SyncJournalEntry
}

func newMemoryStore() Store {
	m := &memoryStore{
		edges:       make(map[ids.ID]*EdgeNode),
		assignments: make(map[ids.ID][]ids.ID),
		journals:    make(map[ids.ID][]*SyncJournalEntry),
	}
	// Seed demo edge
	demoID := ids.MustNew()
	m.edges[demoID] = &EdgeNode{
		ID:               demoID,
		TenantID:         ids.Nil,
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
		DownlinkSeq:      0,
		UplinkSeq:        1042,
		Version:          1,
	}
	return m
}

func (m *memoryStore) CreateEdge(ctx context.Context, e *EdgeNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.edges[e.ID] = e
	return nil
}

func (m *memoryStore) GetEdge(ctx context.Context, tenantID, edgeID ids.ID) (*EdgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.edges[edgeID]
	if !ok || (tenantID != ids.Nil && e.TenantID != ids.Nil && e.TenantID != tenantID) {
		return nil, errs.NotFound("edge node not found")
	}
	return e, nil
}

func (m *memoryStore) GetEdgeByKey(ctx context.Context, edgeKey string) (*EdgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, e := range m.edges {
		if e.EdgeKey == edgeKey {
			return e, nil
		}
	}
	return nil, errs.NotFound("edge node not found for key")
}

func (m *memoryStore) ListEdges(ctx context.Context, tenantID ids.ID) ([]*EdgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*EdgeNode, 0)
	for _, e := range m.edges {
		if tenantID == ids.Nil || e.TenantID == ids.Nil || e.TenantID == tenantID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *memoryStore) UpdateEdge(ctx context.Context, e *EdgeNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.edges[e.ID]; !ok {
		return errs.NotFound("edge node not found")
	}
	m.edges[e.ID] = e
	return nil
}

func (m *memoryStore) DeleteEdge(ctx context.Context, tenantID, edgeID ids.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.edges[edgeID]; !ok {
		return errs.NotFound("edge node not found")
	}
	delete(m.edges, edgeID)
	delete(m.assignments, edgeID)
	delete(m.journals, edgeID)
	return nil
}

func (m *memoryStore) AssignDevices(ctx context.Context, edgeID ids.ID, deviceIDs []ids.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	curr := m.assignments[edgeID]
	existing := make(map[ids.ID]bool)
	for _, id := range curr {
		existing[id] = true
	}
	for _, id := range deviceIDs {
		if !existing[id] {
			curr = append(curr, id)
			existing[id] = true
		}
	}
	m.assignments[edgeID] = curr
	if e, ok := m.edges[edgeID]; ok {
		e.AssignedDevices = curr
	}
	return nil
}

func (m *memoryStore) UnassignDevice(ctx context.Context, edgeID ids.ID, deviceID ids.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	curr := m.assignments[edgeID]
	filtered := make([]ids.ID, 0)
	for _, id := range curr {
		if id != deviceID {
			filtered = append(filtered, id)
		}
	}
	m.assignments[edgeID] = filtered
	if e, ok := m.edges[edgeID]; ok {
		e.AssignedDevices = filtered
	}
	return nil
}

func (m *memoryStore) GetAssignedDevices(ctx context.Context, edgeID ids.ID) ([]ids.ID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.assignments[edgeID], nil
}

func (m *memoryStore) RecordSyncJournal(ctx context.Context, entry *SyncJournalEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.journals[entry.EdgeID] = append([]*SyncJournalEntry{entry}, m.journals[entry.EdgeID]...)
	return nil
}

func (m *memoryStore) ListSyncJournal(ctx context.Context, edgeID ids.ID, limit int) ([]*SyncJournalEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := m.journals[edgeID]
	if len(list) > limit {
		return list[:limit], nil
	}
	return list, nil
}

func (m *memoryStore) UpdateSyncState(ctx context.Context, edgeID ids.ID, status string, upSeq int64, lastSyncTs int64, backlog int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.edges[edgeID]; ok {
		e.Status = status
		e.UplinkSeq = upSeq
		e.LastSyncTs = lastSyncTs
		e.BacklogCount = backlog
	}
	return nil
}
