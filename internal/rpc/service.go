package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// DownlinkDispatcher is implemented by transports (like MQTT) to push commands down to devices.
type DownlinkDispatcher interface {
	DispatchRpcDownlink(ctx context.Context, deviceID ids.ID, commandID ids.ID, method string, params json.RawMessage) (bool, error)
}

// Service manages downlink remote procedure calls to edge devices.
type Service interface {
	SendOneWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage) (*model.RpcCommand, error)
	SendTwoWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage, timeout time.Duration) (*model.RpcCommand, error)
	GetRpcCommand(ctx context.Context, tenantID, commandID ids.ID) (*model.RpcCommand, error)
	GetPendingDeviceRpc(ctx context.Context, deviceID ids.ID) (*model.RpcCommand, error)
	DeliverRpcResponse(ctx context.Context, deviceID, commandID ids.ID, response json.RawMessage) error
	DeliverQueuedRpc(ctx context.Context, deviceID ids.ID) ([]model.RpcCommand, error)
	ListDeviceRpcHistory(ctx context.Context, tenantID, deviceID ids.ID, page, pageSize int) ([]model.RpcCommand, int, error)
	DeleteRpcCommand(ctx context.Context, tenantID, commandID ids.ID) error
	SetDispatcher(dispatcher DownlinkDispatcher)
}

type waiter struct {
	ch   chan *model.RpcCommand
	done bool
}

// DefaultService implements Service with optional persistent PostgresStore and memory caching.
type DefaultService struct {
	store      Store
	dispatcher DownlinkDispatcher
	logger     *slog.Logger

	mu       sync.RWMutex
	waiters  map[string]*waiter
	commands map[string]*model.RpcCommand // Memory cache / fallback
}

// NewDefaultService returns an initialized RPC Service.
func NewDefaultService(store Store, logger *slog.Logger) *DefaultService {
	if logger == nil {
		logger = slog.Default()
	}
	s := &DefaultService{
		store:    store,
		logger:   logger,
		waiters:  make(map[string]*waiter),
		commands: make(map[string]*model.RpcCommand),
	}

	// Background sweep for expired RPC commands
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if s.store != nil {
				if swept, err := s.store.SweepExpired(context.Background()); err == nil && swept > 0 {
					s.logger.Debug("swept expired persistent RPC commands", "count", swept)
				}
			}
		}
	}()

	return s
}

// NewMemoryService returns an in-memory Downlink RPC Service.
func NewMemoryService() Service {
	return NewDefaultService(nil, nil)
}

func (s *DefaultService) SetDispatcher(dispatcher DownlinkDispatcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher = dispatcher
}

func (s *DefaultService) SendOneWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage) (*model.RpcCommand, error) {
	now := time.Now().UnixMilli()
	cmd := &model.RpcCommand{
		ID:             ids.MustNew(),
		TenantID:       tenantID,
		DeviceID:       deviceID,
		Method:         method,
		Params:         params,
		TwoWay:         false,
		Status:         model.RpcStatusQueued,
		MaxRetries:     3,
		ExpirationTime: now + (24 * 3600 * 1000), // 24h default expiration
		CreatedTime:    now,
	}

	s.mu.RLock()
	disp := s.dispatcher
	s.mu.RUnlock()

	// Try immediate downlink delivery if device is connected
	if disp != nil {
		delivered, err := disp.DispatchRpcDownlink(ctx, deviceID, cmd.ID, method, params)
		if err == nil && delivered {
			cmd.Status = model.RpcStatusDelivered
		}
	}

	if s.store != nil {
		if err := s.store.CreateCommand(ctx, cmd); err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	s.commands[cmd.ID.String()] = cmd
	s.mu.Unlock()

	return cmd, nil
}

func (s *DefaultService) SendTwoWayRpc(ctx context.Context, tenantID, deviceID ids.ID, method string, params json.RawMessage, timeout time.Duration) (*model.RpcCommand, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	now := time.Now().UnixMilli()
	cmd := &model.RpcCommand{
		ID:             ids.MustNew(),
		TenantID:       tenantID,
		DeviceID:       deviceID,
		Method:         method,
		Params:         params,
		TwoWay:         true,
		Status:         model.RpcStatusQueued,
		MaxRetries:     3,
		ExpirationTime: now + timeout.Milliseconds(),
		CreatedTime:    now,
	}

	w := &waiter{ch: make(chan *model.RpcCommand, 1)}

	s.mu.Lock()
	s.commands[cmd.ID.String()] = cmd
	s.waiters[cmd.ID.String()] = w
	disp := s.dispatcher
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.waiters, cmd.ID.String())
		s.mu.Unlock()
	}()

	// Persist initial record
	if s.store != nil {
		_ = s.store.CreateCommand(ctx, cmd)
	}

	// Try immediate downlink delivery
	if disp != nil {
		delivered, err := disp.DispatchRpcDownlink(ctx, deviceID, cmd.ID, method, params)
		if err == nil && delivered {
			cmd.Status = model.RpcStatusDelivered
			if s.store != nil {
				_ = s.store.UpdateCommand(ctx, tenantID, cmd.ID, model.RpcStatusDelivered, nil)
			}
		}
	}

	select {
	case <-ctx.Done():
		s.mu.Lock()
		cmd.Status = model.RpcStatusFailed
		s.mu.Unlock()
		if s.store != nil {
			_ = s.store.UpdateCommand(context.Background(), tenantID, cmd.ID, model.RpcStatusFailed, nil)
		}
		return nil, ctx.Err()

	case <-time.After(timeout):
		s.mu.Lock()
		cmd.Status = model.RpcStatusTimeout
		s.mu.Unlock()
		if s.store != nil {
			_ = s.store.UpdateCommand(context.Background(), tenantID, cmd.ID, model.RpcStatusTimeout, nil)
		}
		return cmd, nil

	case completed := <-w.ch:
		return completed, nil
	}
}

func (s *DefaultService) GetRpcCommand(ctx context.Context, tenantID, commandID ids.ID) (*model.RpcCommand, error) {
	if s.store != nil {
		return s.store.GetCommand(ctx, tenantID, commandID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	cmd, ok := s.commands[commandID.String()]
	if !ok || cmd.TenantID != tenantID {
		return nil, errs.NotFound("rpc command not found")
	}
	return cmd, nil
}

func (s *DefaultService) GetPendingDeviceRpc(ctx context.Context, deviceID ids.ID) (*model.RpcCommand, error) {
	if s.store != nil {
		queued, err := s.store.ListQueuedRpc(ctx, deviceID)
		if err == nil && len(queued) > 0 {
			cmd := queued[0]
			_ = s.store.UpdateCommand(ctx, cmd.TenantID, cmd.ID, model.RpcStatusDelivered, nil)
			cmd.Status = model.RpcStatusDelivered
			return &cmd, nil
		}
	}

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

func (s *DefaultService) DeliverRpcResponse(ctx context.Context, deviceID, commandID ids.ID, response json.RawMessage) error {
	s.mu.Lock()
	cmd, ok := s.commands[commandID.String()]
	w, hasWaiter := s.waiters[commandID.String()]
	s.mu.Unlock()

	var tenantID ids.ID
	if ok && cmd.DeviceID == deviceID {
		cmd.Status = model.RpcStatusSuccessful
		cmd.ResponseTime = time.Now().UnixMilli()
		cmd.Response = response
		tenantID = cmd.TenantID
	} else if s.store != nil {
		// Try fetching from database if not in memory
		found, err := s.store.GetCommand(ctx, tenantID, commandID)
		if err == nil && found != nil && found.DeviceID == deviceID {
			tenantID = found.TenantID
			cmd = found
			cmd.Status = model.RpcStatusSuccessful
			cmd.ResponseTime = time.Now().UnixMilli()
			cmd.Response = response
		}
	}

	if cmd == nil {
		return errs.NotFound("rpc command not found for device")
	}

	if s.store != nil && tenantID != ids.Nil {
		_ = s.store.UpdateCommand(ctx, tenantID, commandID, model.RpcStatusSuccessful, response)
	}

	if hasWaiter && w != nil && !w.done {
		w.done = true
		w.ch <- cmd
	}

	return nil
}

func (s *DefaultService) DeliverQueuedRpc(ctx context.Context, deviceID ids.ID) ([]model.RpcCommand, error) {
	s.mu.RLock()
	disp := s.dispatcher
	s.mu.RUnlock()

	if disp == nil {
		return nil, nil
	}

	var queued []model.RpcCommand
	if s.store != nil {
		var err error
		queued, err = s.store.ListQueuedRpc(ctx, deviceID)
		if err != nil {
			return nil, err
		}
	} else {
		s.mu.RLock()
		for _, cmd := range s.commands {
			if cmd.DeviceID == deviceID && cmd.Status == model.RpcStatusQueued {
				queued = append(queued, *cmd)
			}
		}
		s.mu.RUnlock()
	}

	delivered := make([]model.RpcCommand, 0, len(queued))
	for _, cmd := range queued {
		ok, err := disp.DispatchRpcDownlink(ctx, deviceID, cmd.ID, cmd.Method, cmd.Params)
		if err == nil && ok {
			cmd.Status = model.RpcStatusDelivered
			if s.store != nil {
				_ = s.store.UpdateCommand(ctx, cmd.TenantID, cmd.ID, model.RpcStatusDelivered, nil)
			}
			delivered = append(delivered, cmd)
		}
	}

	return delivered, nil
}

func (s *DefaultService) ListDeviceRpcHistory(ctx context.Context, tenantID, deviceID ids.ID, page, pageSize int) ([]model.RpcCommand, int, error) {
	if s.store != nil {
		return s.store.ListDeviceRpcHistory(ctx, tenantID, deviceID, page, pageSize)
	}

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

func (s *DefaultService) DeleteRpcCommand(ctx context.Context, tenantID, commandID ids.ID) error {
	if s.store != nil {
		if err := s.store.DeleteCommand(ctx, tenantID, commandID); err != nil {
			return err
		}
	}

	s.mu.Lock()
	delete(s.commands, commandID.String())
	s.mu.Unlock()

	return nil
}
