package core_test

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceManagementAndAuth(t *testing.T) {
	store := core.NewMemoryDeviceStore()
	mgr := core.NewDeviceManager(store, []byte("test-pepper-12345"))

	ctx := context.Background()
	tenant1 := ids.MustNew()
	tenant2 := ids.MustNew()

	// 1. Create device for Tenant 1
	dev1, creds1, err := mgr.CreateDevice(ctx, tenant1, "Thermostat-01", "Office Thermostat", "")
	require.NoError(t, err)
	assert.NotEmpty(t, dev1.ID)
	assert.Equal(t, "Thermostat-01", dev1.Name)
	assert.NotEmpty(t, creds1.CredentialsValue)

	// 2. Fetch device
	fetched, err := mgr.GetDevice(ctx, tenant1, dev1.ID)
	require.NoError(t, err)
	assert.Equal(t, dev1.ID, fetched.ID)

	// 3. Multi-tenancy test: Tenant 2 cannot access Tenant 1's device
	_, err = mgr.GetDevice(ctx, tenant2, dev1.ID)
	assert.Error(t, err, "Tenant 2 should get NotFound for Tenant 1 device")

	// 4. Authenticate device token
	authCreds, err := mgr.AuthenticateToken(ctx, creds1.CredentialsValue)
	require.NoError(t, err)
	assert.Equal(t, dev1.ID, authCreds.DeviceID)
	assert.Equal(t, tenant1, authCreds.TenantID)

	// 5. Invalid token fails authentication
	_, err = mgr.AuthenticateToken(ctx, "invalid-token-xyz")
	assert.Error(t, err)

	// 6. List devices scoped by tenant
	list1, total1, err := mgr.ListDevices(ctx, tenant1, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total1)
	assert.Len(t, list1, 1)

	list2, total2, err := mgr.ListDevices(ctx, tenant2, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total2)
	assert.Empty(t, list2)
}
