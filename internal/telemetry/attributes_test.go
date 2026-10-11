package telemetry_test

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttributeStore(t *testing.T) {
	ctx := context.Background()
	store := telemetry.NewMemoryAttributeStore()

	devID := ids.MustNew()

	// 1. Save Client Attributes
	err := store.SaveAttributes(ctx, devID, model.ScopeClient, map[string]any{
		"firmware": "v1.0.0",
		"ip":       "10.0.0.1",
	})
	require.NoError(t, err)

	// 2. Save Shared Attributes
	err = store.SaveAttributes(ctx, devID, model.ScopeShared, map[string]any{
		"interval": 10,
	})
	require.NoError(t, err)

	// 3. Query Client scope
	clientAttrs, err := store.GetAttributes(ctx, devID, model.ScopeClient, []string{"firmware"})
	require.NoError(t, err)
	assert.Len(t, clientAttrs, 1)
	assert.Equal(t, "v1.0.0", clientAttrs[0].Val)

	// 4. Query All attributes across scopes
	all, err := store.GetAllAttributes(ctx, devID)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// 5. Delete attribute
	err = store.DeleteAttributes(ctx, devID, model.ScopeClient, []string{"ip"})
	require.NoError(t, err)

	clientAttrsAfter, err := store.GetAttributes(ctx, devID, model.ScopeClient, nil)
	require.NoError(t, err)
	assert.Len(t, clientAttrsAfter, 1)
}
