package rpc

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Service manages downlink remote procedure calls to edge devices.
type Service interface {
	SendOneWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage) (*model.RpcCommand, error)
	SendTwoWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage, timeout time.Duration) (*model.RpcCommand, error)
	GetPendingDeviceRpc(ctx context.Context, deviceID ids.ID) (*model.RpcCommand, error)
	DeliverRpcResponse(ctx context.Context, deviceID, commandID ids.ID, response json.RawMessage) error
	ListDeviceRpcHistory(ctx context.Context, tenantID, deviceID ids.ID, page, pageSize int) ([]model.RpcCommand, int, error)
}

type waiter struct {
	ch   chan *model.RpcCommand
	done bool
}

type memoryService struct {
	mu       sync.RWMutex
	commands map[string]*model.RpcCommand
	waiters  map[string]*waiter
}

// NewMemoryService returns an in-memory Downlink RPC Service.
func NewMemoryService() Service {
	return &memoryService{
		commands: make(map[string]*model.RpcCommand),
		waiters:  make(map[string]*waiter),
	}
}

func (s *memoryService) SendOneWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage) (*model.RpcCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := &model.RpcCommand{
		ID:          ids.MustNew(),
		TenantID:    tenantID,
		DeviceID:    deviceID,
		Method:      method,
		Params:      params,
		TwoWay:      false,
		Status:      model.RpcStatusQueued,
		CreatedTime: time.Now().UnixMilli(),
	}

	s.commands[cmd.ID.String()] = cmd
	return cmd, nil
}

func (s *memoryService) SendTwoWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage, timeout time.Duration) (*model.RpcCommand, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	cmd := &model.RpcCommand{
		ID:          ids.MustNew(),
		TenantID:    tenantID,
		DeviceID:    deviceID,
		Method:      method,
		Params:      params,
		TwoWay:      true,
		Status:      model.RpcStatusQueued,
		CreatedTime: time.Now().UnixMilli(),
	}

	w := &waiter{ch: make(chan *model.RpcCommand, 1)}

	s.mu.Lock()
	s.commands[cmd.ID.String()] = cmd
	s.waiters[cmd.ID.String()] = w
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.waiters, cmd.ID.String())
		s.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		cmd.Status = model.RpcStatusFailed
		s.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(timeout):
		s.mu.Lock()
		cmd.Status = model.RpcStatusTimeout
		s.mu.Unlock()
		return cmd, nil
	case completed := <-w.ch:
		return completed, nil
	}
}

func (s *memoryService) GetPendingDeviceRpc(ctx context.Context, deviceID ids.ID) (*model.RpcCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, cmd := range s.commands {
		if cmd.DeviceID == deviceID && cmd.Status == model.RpcStatusQueued {
			cmd.Status = model.RpcStatusDelivered
			return cmd, nil
		}
	}
	return nil, nil
}

func (s *memoryService) DeliverRpcResponse(ctx context.Context, deviceID, commandID ids.ID, response json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd, ok := s.commands[commandID.String()]
	if !ok || cmd.DeviceID != deviceID {
		return errs.NotFound("rpc command not found for device")
	}

	cmd.Status = model.RpcStatusSuccessful
	cmd.ResponseTime = time.Now().UnixMilli()
	cmd.Response = response

	if w, ok := s.waiters[commandID.String()]; ok && !w.done {
		w.done = true
		w.ch <- cmd
	}

	return nil
}

func (s *memoryService) ListDeviceRpcHistory(ctx context.Context, tenantID, deviceID ids.ID, page, pageSize int) ([]model.RpcCommand, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	var list []model.RpcCommand
	for _, cmd := range s.commands {
		if cmd.TenantID == tenantID && cmd.DeviceID == deviceID {
			list = append(list, *cmd)
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedTime > list[j].CreatedTime
	})

	total := len(list)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []model.RpcCommand{}, total, nil
	}

	end := offset + pageSize
	if end > total {
		end = total
	}

	return list[offset:end], total, nil
}
