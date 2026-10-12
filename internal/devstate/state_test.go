package devstate_test

import (
	"context"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestConnectAndDisconnect(t *testing.T) {
	ctx := context.Background()
	tracker := devstate.NewMemoryTracker(100 * time.Millisecond)
	devID := ids.MustNew()

	// Connect
	now := time.Now().UnixMilli()
	tracker.RecordConnect(ctx, devID, now)
	st, err := tracker.GetState(ctx, devID)
	require.NoError(t, err)
	assert.True(t, st.Active)
	assert.Equal(t, now, st.LastConnectTime)

	// Disconnect
	discTime := now + 50
	tracker.RecordDisconnect(ctx, devID, discTime)
	st, err = tracker.GetState(ctx, devID)
	require.NoError(t, err)
	assert.Equal(t, discTime, st.LastDisconnectTime)
}

func TestOverrideAndSummary(t *testing.T) {
	ctx := context.Background()
	tracker := devstate.NewMemoryTracker(10 * time.Second)
	tenantID := ids.MustNew()
	dev1 := ids.MustNew()
	dev2 := ids.MustNew()

	tracker.RegisterDevice(ctx, tenantID, dev1, 10000)
	tracker.RegisterDevice(ctx, tenantID, dev2, 10000)

	// Activate dev1
	tracker.RecordActivity(ctx, dev1, time.Now().UnixMilli())

	// Override dev2 as active
	_, err := tracker.SetOverride(ctx, dev2, true)
	require.NoError(t, err)

	sum, err := tracker.GetSummary(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, 2, sum.Total)
	assert.Equal(t, 2, sum.Active)
	assert.Equal(t, 0, sum.Inactive)

	// Override dev1 as inactive
	_, err = tracker.SetOverride(ctx, dev1, false)
	require.NoError(t, err)
	sum, err = tracker.GetSummary(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Active)
	assert.Equal(t, 1, sum.Inactive)
}

func TestInactivityAlarmAndAttributes(t *testing.T) {
	ctx := context.Background()
	attrs := telemetry.NewMemoryAttributeStore()
	alarms := alarm.NewMemoryService()

	tracker := devstate.NewEngine(50*time.Millisecond, attrs, alarms)
	tenantID := ids.MustNew()
	devID := ids.MustNew()
	tracker.RegisterDevice(ctx, tenantID, devID, 50)

	// Record connect
	tracker.RecordConnect(ctx, devID, time.Now().UnixMilli())

	// Verify server-scoped attributes persisted
	srvAttrs, err := attrs.GetAttributes(ctx, devID, model.ScopeServer, []string{"active", "lastConnectTime"})
	require.NoError(t, err)
	assert.NotEmpty(t, srvAttrs)

	var activeVal any
	for _, a := range srvAttrs {
		if a.Key == "active" {
			activeVal = a.Val
		}
	}
	assert.Equal(t, true, activeVal)
}
