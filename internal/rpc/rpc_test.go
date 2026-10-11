package rpc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rpc"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOneWayRpc(t *testing.T) {
	ctx := context.Background()
	svc := rpc.NewMemoryService()

	tenantID := ids.MustNew()
	devID := ids.MustNew()

	cmd, err := svc.SendOneWayRpc(ctx, tenantID, devID, "reboot", json.RawMessage(`{"delay": 5}`))
	require.NoError(t, err)
	assert.Equal(t, model.RpcStatusQueued, cmd.Status)
	assert.False(t, cmd.TwoWay)

	// Device polls for pending RPC
	pending, err := svc.GetPendingDeviceRpc(ctx, devID)
	require.NoError(t, err)
	require.NotNil(t, pending)
	assert.Equal(t, cmd.ID, pending.ID)
	assert.Equal(t, model.RpcStatusDelivered, pending.Status)
}

func TestTwoWayRpcDelivery(t *testing.T) {
	ctx := context.Background()
	svc := rpc.NewMemoryService()

	tenantID := ids.MustNew()
	devID := ids.MustNew()

	// Simulate device worker answering the RPC in a goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		pending, err := svc.GetPendingDeviceRpc(context.Background(), devID)
		if err == nil && pending != nil {
			_ = svc.DeliverRpcResponse(context.Background(), devID, pending.ID, json.RawMessage(`{"success":true,"relay":1}`))
		}
	}()

	cmd, err := svc.SendTwoWayRpc(ctx, tenantID, devID, "setRelay", json.RawMessage(`{"relay":1,"state":true}`), 2*time.Second)
	require.NoError(t, err)
	assert.Equal(t, model.RpcStatusSuccessful, cmd.Status)
	assert.JSONEq(t, `{"success":true,"relay":1}`, string(cmd.Response))
}
