package alarm_test

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlarmLifecycle(t *testing.T) {
	ctx := context.Background()
	svc := alarm.NewMemoryService()

	tenantID := ids.MustNew()
	devID := ids.MustNew()

	// 1. Create Alarm
	a1, err := svc.CreateOrUpdateAlarm(ctx, &model.Alarm{
		TenantID:     tenantID,
		OriginatorID: devID,
		Type:         "High Temperature Alert",
		Severity:     model.SeverityCritical,
	})
	require.NoError(t, err)
	assert.Equal(t, model.AlarmActiveUnack, a1.Status)
	assert.Equal(t, model.SeverityCritical, a1.Severity)

	// 2. Deduplicate existing active alarm
	a2, err := svc.CreateOrUpdateAlarm(ctx, &model.Alarm{
		TenantID:     tenantID,
		OriginatorID: devID,
		Type:         "High Temperature Alert",
		Severity:     model.SeverityCritical,
	})
	require.NoError(t, err)
	assert.Equal(t, a1.ID, a2.ID, "should deduplicate and update existing active alarm")

	// 3. Acknowledge Alarm
	acked, err := svc.AckAlarm(ctx, tenantID, a1.ID)
	require.NoError(t, err)
	assert.Equal(t, model.AlarmActiveAck, acked.Status)
	assert.True(t, acked.AckTs > 0)

	// 4. Clear Alarm
	cleared, err := svc.ClearAlarm(ctx, tenantID, a1.ID)
	require.NoError(t, err)
	assert.Equal(t, model.AlarmClearedAck, cleared.Status)
	assert.True(t, cleared.ClearTs > 0)

	// 5. Tenant Isolation
	otherTenant := ids.MustNew()
	_, err = svc.GetAlarm(ctx, otherTenant, a1.ID)
	assert.Error(t, err, "other tenant should not access alarm")
}
