package telemetry_test

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePayloadVariants(t *testing.T) {
	// 1. Plain map
	p1 := []byte(`{"temperature": 22.5, "humidity": 60, "active": true}`)
	pts1, err := telemetry.ParsePayload(p1, 1000)
	require.NoError(t, err)
	assert.Len(t, pts1, 3)

	// 2. Object with ts and values
	p2 := []byte(`{"ts": 1730000000000, "values": {"temperature": 25.1}}`)
	pts2, err := telemetry.ParsePayload(p2, 1000)
	require.NoError(t, err)
	require.Len(t, pts2, 1)
	assert.Equal(t, int64(1730000000000), pts2[0].Timestamp)
	assert.Equal(t, "temperature", pts2[0].Key)
	assert.InDelta(t, 25.1, *pts2[0].DblV, 0.001)

	// 3. Batch array
	p3 := []byte(`[{"ts": 100, "values": {"t": 20}}, {"ts": 200, "values": {"t": 21}}]`)
	pts3, err := telemetry.ParsePayload(p3, 1000)
	require.NoError(t, err)
	assert.Len(t, pts3, 2)
	assert.Equal(t, int64(100), pts3[0].Timestamp)
	assert.Equal(t, int64(200), pts3[1].Timestamp)
}

func TestTelemetryStoreLatestAndTimeseries(t *testing.T) {
	store := telemetry.NewMemoryStore()
	ctx := context.Background()
	deviceID := ids.MustNew()

	pts := []*telemetry.TelemetryDataPoint{
		{Key: "temp", Timestamp: 1000, DblV: floatPtr(20.0)},
		{Key: "temp", Timestamp: 2000, DblV: floatPtr(21.5)},
		{Key: "temp", Timestamp: 3000, DblV: floatPtr(22.0)},
		{Key: "humidity", Timestamp: 3000, DblV: floatPtr(55.0)},
	}

	err := store.Save(ctx, deviceID, pts)
	require.NoError(t, err)

	// Latest query
	latest, err := store.GetLatest(ctx, deviceID, []string{"temp", "humidity"})
	require.NoError(t, err)
	require.NotNil(t, latest["temp"])
	assert.InDelta(t, 22.0, *latest["temp"].DblV, 0.001)
	assert.Equal(t, int64(3000), latest["temp"].Timestamp)

	// Timeseries query
	tsData, err := store.GetTimeseries(ctx, deviceID, []string{"temp"}, 1000, 2500, 10)
	require.NoError(t, err)
	require.Len(t, tsData["temp"], 2)
	assert.Equal(t, int64(1000), tsData["temp"][0].Timestamp)
	assert.Equal(t, int64(2000), tsData["temp"][1].Timestamp)
}

func floatPtr(f float64) *float64 {
	return &f
}
