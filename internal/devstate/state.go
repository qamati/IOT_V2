package devstate

import (
	"context"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Tracker manages device online/offline states and activity timestamps.
type Tracker interface {
	RecordActivity(ctx context.Context, deviceID ids.ID, ts int64)
	GetActivity(ctx context.Context, deviceID ids.ID) model.DeviceActivity
	IsActive(ctx context.Context, deviceID ids.ID) bool
}

type memoryTracker struct {
	mu              sync.RWMutex
	inactivityLimit time.Duration
	activities      map[string]int64
}

// NewMemoryTracker creates a device activity tracker with a given inactivity timeout.
func NewMemoryTracker(inactivityTimeout time.Duration) Tracker {
	if inactivityTimeout <= 0 {
		inactivityTimeout = 60 * time.Second
	}
	return &memoryTracker{
		inactivityLimit: inactivityTimeout,
		activities:      make(map[string]int64),
	}
}

func (t *memoryTracker) RecordActivity(ctx context.Context, deviceID ids.ID, ts int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	t.activities[deviceID.String()] = ts
}

func (t *memoryTracker) GetActivity(ctx context.Context, deviceID ids.ID) model.DeviceActivity {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ts, ok := t.activities[deviceID.String()]
	if !ok {
		return model.DeviceActivity{
			DeviceID:         deviceID,
			LastActivityTime: 0,
			Active:           false,
		}
	}

	diff := time.Now().UnixMilli() - ts
	active := diff < t.inactivityLimit.Milliseconds()

	return model.DeviceActivity{
		DeviceID:         deviceID,
		LastActivityTime: ts,
		Active:           active,
	}
}

func (t *memoryTracker) IsActive(ctx context.Context, deviceID ids.ID) bool {
	return t.GetActivity(ctx, deviceID).Active
}
