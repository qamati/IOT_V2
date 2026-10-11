package ids_test

import (
	"testing"
	"time"

	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUIDv7Generation(t *testing.T) {
	nowMs := time.Now().UnixMilli()

	id, err := ids.New()
	require.NoError(t, err)
	assert.NotEqual(t, ids.Nil, id)
	assert.Equal(t, 7, int(id.Version()))

	ts := ids.TimestampMs(id)
	assert.InDelta(t, nowMs, ts, 500, "Timestamp should be close to current time")

	tm := ids.Time(id)
	assert.InDelta(t, nowMs, tm.UnixMilli(), 500)
}

func TestTimeOrdering(t *testing.T) {
	id1 := ids.MustNew()
	time.Sleep(5 * time.Millisecond)
	id2 := ids.MustNew()

	assert.True(t, ids.TimestampMs(id2) >= ids.TimestampMs(id1))
	assert.True(t, id1.String() < id2.String(), "UUIDv7 strings should be chronologically sortable")
}

func TestParseAndValidate(t *testing.T) {
	id := ids.MustNew()
	s := id.String()

	assert.True(t, ids.IsValid(s))
	assert.False(t, ids.IsValid("not-a-uuid"))

	parsed, err := ids.Parse(s)
	require.NoError(t, err)
	assert.Equal(t, id, parsed)

	mustParsed := ids.MustParse(s)
	assert.Equal(t, id, mustParsed)
}
