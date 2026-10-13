package edge

import (
	"context"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// Store defines persistence operations for edge nodes, assignments, and sync journals.
type Store interface {
	CreateEdge(ctx context.Context, e *EdgeNode) error
	GetEdge(ctx context.Context, tenantID, edgeID ids.ID) (*EdgeNode, error)
	GetEdgeByKey(ctx context.Context, edgeKey string) (*EdgeNode, error)
	ListEdges(ctx context.Context, tenantID ids.ID) ([]*EdgeNode, error)
	UpdateEdge(ctx context.Context, e *EdgeNode) error
	DeleteEdge(ctx context.Context, tenantID, edgeID ids.ID) error

	AssignDevices(ctx context.Context, edgeID ids.ID, deviceIDs []ids.ID) error
	UnassignDevice(ctx context.Context, edgeID ids.ID, deviceID ids.ID) error
	GetAssignedDevices(ctx context.Context, edgeID ids.ID) ([]ids.ID, error)

	RecordSyncJournal(ctx context.Context, entry *SyncJournalEntry) error
	ListSyncJournal(ctx context.Context, edgeID ids.ID, limit int) ([]*SyncJournalEntry, error)
	UpdateSyncState(ctx context.Context, edgeID ids.ID, status string, upSeq int64, lastSyncTs int64, backlog int64) error
}
