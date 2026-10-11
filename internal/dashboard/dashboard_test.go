package dashboard_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qamati/IOT_V2/internal/dashboard"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardCRUD(t *testing.T) {
	ctx := context.Background()
	svc := dashboard.NewMemoryService()

	tenantA := ids.MustNew()
	tenantB := ids.MustNew()

	// 1. Create Dashboard
	cfg := json.RawMessage(`{"widgets":[{"id":"1","type":"chart"}]}`)
	d, err := svc.CreateDashboard(ctx, tenantA, "Production Line 1", cfg)
	require.NoError(t, err)
	assert.Equal(t, "Production Line 1", d.Title)
	assert.Equal(t, tenantA, d.TenantID)

	// 2. Read Dashboard
	got, err := svc.GetDashboard(ctx, tenantA, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d.ID, got.ID)

	// 3. Multi-Tenant Isolation
	_, err = svc.GetDashboard(ctx, tenantB, d.ID)
	assert.Error(t, err, "Tenant B should not see Tenant A dashboard")

	// 4. Update Dashboard
	updated, err := svc.UpdateDashboard(ctx, tenantA, d.ID, "Production Line 1 (Updated)", nil)
	require.NoError(t, err)
	assert.Equal(t, "Production Line 1 (Updated)", updated.Title)

	// 5. Delete Dashboard
	err = svc.DeleteDashboard(ctx, tenantA, d.ID)
	require.NoError(t, err)

	_, err = svc.GetDashboard(ctx, tenantA, d.ID)
	assert.Error(t, err, "dashboard should be deleted")
}
