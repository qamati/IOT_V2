package devstate_test

import (
	"context"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
)

func TestDevStateTracking(t *testing.T) {
	ctx := context.Background()
	tracker := devstate.NewMemoryTracker(50 * time.Millisecond)

	devID := ids.MustNew()

	// Initially inactive
	assert.False(t, tracker.IsActive(ctx, devID))

	// Record activity
	now := time.Now().UnixMilli()
	tracker.RecordActivity(ctx, devID, now)

	// Now active
	assert.True(t, tracker.IsActive(ctx, devID))
	act := tracker.GetActivity(ctx, devID)
	assert.Equal(t, now, act.LastActivityTime)
	assert.True(t, act.Active)

	// Wait beyond timeout (60ms)
	time.Sleep(60 * time.Millisecond)
	assert.False(t, tracker.IsActive(ctx, devID), "should flip to inactive after timeout")
}
