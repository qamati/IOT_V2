package rule_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleEngineThresholds(t *testing.T) {
	ctx := context.Background()
	alarmSvc := alarm.NewMemoryService()
	engine := rule.NewEngine(alarmSvc, slog.Default())

	tenantID := ids.MustNew()
	devID := ids.MustNew()

	// High temp point: 48.5°C
	tempVal := 48.5
	points := []*model.TelemetryDataPoint{
		{
			Key:       "temperature",
			Timestamp: 1000,
			DblV:      &tempVal,
		},
	}

	engine.EvaluateTelemetry(ctx, tenantID, devID, points)

	// Verify Alarm was created
	alarms, total, err := alarmSvc.ListAlarms(ctx, tenantID, &devID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "High Temperature Alert", alarms[0].Type)
	assert.Equal(t, model.SeverityCritical, alarms[0].Severity)
}
