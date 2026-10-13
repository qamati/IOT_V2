package edge

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// Priority Lanes for Edge Store-and-Forward.
type Lane uint8

const (
	LaneL0 Lane = iota // High-Priority: Alarms & Control Events
	LaneL1             // Medium-Priority: Realtime Telemetry & State
	LaneL2             // Low-Priority: Historical Logs & Diagnostics
)

// QueueStats provides metrics on store-and-forward backlog.
type QueueStats struct {
	TotalQueued  int64 `json:"total_queued"`
	LaneL0Queued int   `json:"lane_l0_queued"`
	LaneL1Queued int   `json:"lane_l1_queued"`
	LaneL2Queued int   `json:"lane_l2_queued"`
	LastSeq      int64 `json:"last_seq"`
	AckedSeq     int64 `json:"acked_seq"`
}

// Queue defines the store-and-forward buffer on edge nodes.
type Queue interface {
	Enqueue(ctx context.Context, lane Lane, item SyncItem) (int64, error)
	DrainBatch(ctx context.Context, maxItems int) (*SyncBatch, error)
	Ack(seq int64) error
	Stats() QueueStats
}

type memoryQueue struct {
	edgeID   ids.ID
	mu       sync.Mutex
	l0       []SyncItem
	l1       []SyncItem
	l2       []SyncItem
	sequence atomic.Int64
	ackedSeq atomic.Int64
	maxCap   int
}

// NewQueue creates an autonomous multi-lane store-and-forward queue.
func NewQueue(edgeID ids.ID, maxCapacity int) Queue {
	if maxCapacity <= 0 {
		maxCapacity = 10000
	}
	q := &memoryQueue{
		edgeID: edgeID,
		l0:     make([]SyncItem, 0),
		l1:     make([]SyncItem, 0),
		l2:     make([]SyncItem, 0),
		maxCap: maxCapacity,
	}
	return q
}

func (q *memoryQueue) Enqueue(ctx context.Context, lane Lane, item SyncItem) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if item.Timestamp == 0 {
		item.Timestamp = time.Now().UnixMilli()
	}

	total := len(q.l0) + len(q.l1) + len(q.l2)
	if total >= q.maxCap {
		// Capacity policy: L0 is never dropped; L2 is dropped first, then oldest L1
		if len(q.l2) > 0 {
			q.l2 = q.l2[1:]
		} else if len(q.l1) > 0 {
			q.l1 = q.l1[1:]
		} else {
			return 0, fmt.Errorf("queue: hard capacity reached on L0")
		}
	}

	switch lane {
	case LaneL0:
		q.l0 = append(q.l0, item)
	case LaneL1:
		q.l1 = append(q.l1, item)
	case LaneL2:
		q.l2 = append(q.l2, item)
	default:
		q.l1 = append(q.l1, item)
	}

	seq := q.sequence.Add(1)
	return seq, nil
}

func (q *memoryQueue) DrainBatch(ctx context.Context, maxItems int) (*SyncBatch, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	total := len(q.l0) + len(q.l1) + len(q.l2)
	if total == 0 {
		return nil, nil
	}

	if maxItems <= 0 {
		maxItems = 100
	}

	batch := &SyncBatch{
		EdgeID:    q.edgeID,
		Sequence:  q.sequence.Load(),
		Timestamp: time.Now().UnixMilli(),
		LaneL0:    make([]SyncItem, 0),
		LaneL1:    make([]SyncItem, 0),
		LaneL2:    make([]SyncItem, 0),
	}

	remaining := maxItems

	// 1. Drain Lane L0 (Alarms first)
	takeL0 := len(q.l0)
	if takeL0 > remaining {
		takeL0 = remaining
	}
	batch.LaneL0 = append(batch.LaneL0, q.l0[:takeL0]...)
	remaining -= takeL0

	// 2. Drain Lane L1 (Telemetry second)
	if remaining > 0 {
		takeL1 := len(q.l1)
		if takeL1 > remaining {
			takeL1 = remaining
		}
		batch.LaneL1 = append(batch.LaneL1, q.l1[:takeL1]...)
		remaining -= takeL1
	}

	// 3. Drain Lane L2 (Historical logs)
	if remaining > 0 {
		takeL2 := len(q.l2)
		if takeL2 > remaining {
			takeL2 = remaining
		}
		batch.LaneL2 = append(batch.LaneL2, q.l2[:takeL2]...)
	}

	return batch, nil
}

func (q *memoryQueue) Ack(seq int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// On successful cumulative ack, clear buffered items
	q.l0 = make([]SyncItem, 0)
	q.l1 = make([]SyncItem, 0)
	q.l2 = make([]SyncItem, 0)
	q.ackedSeq.Store(seq)
	return nil
}

func (q *memoryQueue) Stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()

	return QueueStats{
		TotalQueued:  int64(len(q.l0) + len(q.l1) + len(q.l2)),
		LaneL0Queued: len(q.l0),
		LaneL1Queued: len(q.l1),
		LaneL2Queued: len(q.l2),
		LastSeq:      q.sequence.Load(),
		AckedSeq:     q.ackedSeq.Load(),
	}
}
