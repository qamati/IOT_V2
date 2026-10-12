package rpc_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rpc"
	"github.com/qamati/IOT_V2/pkg/ids"
)

type mockDispatcher struct {
	mu        sync.Mutex
	dispatched []ids.ID
}

func (m *mockDispatcher) DispatchRpcDownlink(ctx context.Context, deviceID ids.ID, commandID ids.ID, method string, params json.RawMessage) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatched = append(m.dispatched, commandID)
	return true, nil
}

func TestOneWayRpc(t *testing.T) {
	svc := rpc.NewMemoryService()
	disp := &mockDispatcher{}
	svc.SetDispatcher(disp)

	tenantID := ids.MustNew()
	deviceID := ids.MustNew()

	cmd, err := svc.SendOneWayRpc(context.Background(), tenantID, deviceID, "reboot", json.RawMessage(`{"delay": 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Method != "reboot" {
		t.Errorf("expected method reboot, got %s", cmd.Method)
	}
	if cmd.Status != model.RpcStatusDelivered {
		t.Errorf("expected status DELIVERED with active dispatcher, got %s", cmd.Status)
	}

	history, total, err := svc.ListDeviceRpcHistory(context.Background(), tenantID, deviceID, 1, 10)
	if err != nil {
		t.Fatalf("unexpected history error: %v", err)
	}
	if total != 1 || len(history) != 1 {
		t.Errorf("expected 1 history item, got total %d len %d", total, len(history))
	}
}

func TestTwoWayRpc_SynchronousResponse(t *testing.T) {
	svc := rpc.NewMemoryService()
	tenantID := ids.MustNew()
	deviceID := ids.MustNew()

	go func() {
		time.Sleep(50 * time.Millisecond)
		pending, err := svc.GetPendingDeviceRpc(context.Background(), deviceID)
		if err != nil || pending == nil {
			return
		}
		_ = svc.DeliverRpcResponse(context.Background(), deviceID, pending.ID, json.RawMessage(`{"temp": 24.5}`))
	}()

	cmd, err := svc.SendTwoWayRpc(context.Background(), tenantID, deviceID, "getTemperature", json.RawMessage(`{}`), 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected two-way error: %v", err)
	}

	if cmd.Status != model.RpcStatusSuccessful {
		t.Errorf("expected status SUCCESSFUL, got %s", cmd.Status)
	}
	if string(cmd.Response) != `{"temp": 24.5}` {
		t.Errorf("expected response `{\"temp\": 24.5}`, got %s", string(cmd.Response))
	}
}

func TestTwoWayRpc_Timeout(t *testing.T) {
	svc := rpc.NewMemoryService()
	tenantID := ids.MustNew()
	deviceID := ids.MustNew()

	cmd, err := svc.SendTwoWayRpc(context.Background(), tenantID, deviceID, "getSlowReport", json.RawMessage(`{}`), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Status != model.RpcStatusTimeout {
		t.Errorf("expected status TIMEOUT, got %s", cmd.Status)
	}
}
