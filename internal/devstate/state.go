package devstate

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/alarm"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// EventType defines device lifecycle state transition events.
type EventType string

const (
	EventConnect    EventType = "CONNECT_EVENT"
	EventDisconnect EventType = "DISCONNECT_EVENT"
	EventActivity   EventType = "ACTIVITY_EVENT"
	EventInactivity EventType = "INACTIVITY_EVENT"
)

// State alias for model.DeviceState.
type State = model.DeviceState

// StateSummary provides tenant-wide device state aggregate metrics.
type StateSummary struct {
	TenantID ids.ID `json:"tenant_id"`
	Total    int    `json:"total"`
	Active   int    `json:"active"`
	Inactive int    `json:"inactive"`
}

// StateListener allows external subscribers (e.g. Rule Engine, WebSocket) to receive state events.
type StateListener func(ctx context.Context, ev EventType, st *State)

// Tracker manages device online/offline states, connect/disconnect lifecycles, and inactivity timeouts.
type Tracker interface {
	RecordActivity(ctx context.Context, deviceID ids.ID, ts int64)
	RecordConnect(ctx context.Context, deviceID ids.ID, ts int64)
	RecordDisconnect(ctx context.Context, deviceID ids.ID, ts int64)
	GetActivity(ctx context.Context, deviceID ids.ID) model.DeviceActivity
	GetState(ctx context.Context, deviceID ids.ID) (*State, error)
	IsActive(ctx context.Context, deviceID ids.ID) bool
	SetOverride(ctx context.Context, deviceID ids.ID, active bool) (*State, error)
	GetSummary(ctx context.Context, tenantID ids.ID) (*StateSummary, error)
	RegisterDevice(ctx context.Context, tenantID, deviceID ids.ID, inactivityTimeoutMs int64)
	AddListener(listener StateListener)
	Start(ctx context.Context)
	Stop()
}

type engine struct {
	mu               sync.RWMutex
	defaultTimeoutMs int64
	states           map[ids.ID]*State
	listeners        []StateListener
	attrStore        telemetry.AttributeStore
	alarmSvc         alarm.Service
	running          bool
	stopCh           chan struct{}
}

// NewMemoryTracker creates a standalone in-memory device state tracker.
func NewMemoryTracker(inactivityTimeout time.Duration) Tracker {
	return NewEngine(inactivityTimeout, nil, nil)
}

// NewEngine creates a full Device State Engine with optional attribute persistence and alarm raising.
func NewEngine(inactivityTimeout time.Duration, attrStore telemetry.AttributeStore, alarmSvc alarm.Service) Tracker {
	if inactivityTimeout <= 0 {
		inactivityTimeout = 60 * time.Second
	}
	return &engine{
		defaultTimeoutMs: inactivityTimeout.Milliseconds(),
		states:           make(map[ids.ID]*State),
		listeners:        make([]StateListener, 0),
		attrStore:        attrStore,
		alarmSvc:         alarmSvc,
		stopCh:           make(chan struct{}),
	}
}

func (e *engine) Start(ctx context.Context) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	go e.loop(ctx)
}

func (e *engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.running = false
	close(e.stopCh)
}

func (e *engine) loop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkInactivity(ctx)
		}
	}
}

func (e *engine) checkInactivity(ctx context.Context) {
	now := time.Now().UnixMilli()
	var transitioned []*State

	e.mu.Lock()
	for _, st := range e.states {
		if st.Active && !st.Overridden && now >= st.Deadline {
			st.Active = false
			st.InactivityAlarmTime = now
			stCopy := *st
			transitioned = append(transitioned, &stCopy)
		}
	}
	e.mu.Unlock()

	for _, st := range transitioned {
		e.notifyListeners(ctx, EventInactivity, st)
		e.persistAttributes(ctx, st)
		e.raiseInactivityAlarm(ctx, st)
	}
}

func (e *engine) RegisterDevice(ctx context.Context, tenantID, deviceID ids.ID, inactivityTimeoutMs int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if inactivityTimeoutMs <= 0 {
		inactivityTimeoutMs = e.defaultTimeoutMs
	}

	st, ok := e.states[deviceID]
	if !ok {
		st = &State{
			DeviceID:            deviceID,
			TenantID:            tenantID,
			InactivityTimeoutMs: inactivityTimeoutMs,
			Active:              false,
		}
		e.states[deviceID] = st
	} else {
		st.TenantID = tenantID
		st.InactivityTimeoutMs = inactivityTimeoutMs
	}
}

func (e *engine) AddListener(listener StateListener) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = append(e.listeners, listener)
}

func (e *engine) notifyListeners(ctx context.Context, ev EventType, st *State) {
	e.mu.RLock()
	list := make([]StateListener, len(e.listeners))
	copy(list, e.listeners)
	e.mu.RUnlock()

	for _, fn := range list {
		fn(ctx, ev, st)
	}
}

func (e *engine) RecordActivity(ctx context.Context, deviceID ids.ID, ts int64) {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	e.mu.Lock()
	st, ok := e.states[deviceID]
	if !ok {
		st = &State{
			DeviceID:            deviceID,
			InactivityTimeoutMs: e.defaultTimeoutMs,
		}
		e.states[deviceID] = st
	}

	wasActive := st.Active && (time.Now().UnixMilli() < st.Deadline || st.Overridden)
	if ts > st.LastActivityTime {
		st.LastActivityTime = ts
	}
	st.Active = true
	st.Deadline = st.LastActivityTime + st.InactivityTimeoutMs
	stCopy := *st
	e.mu.Unlock()

	e.notifyListeners(ctx, EventActivity, &stCopy)
	if !wasActive {
		e.persistAttributes(ctx, &stCopy)
		e.clearInactivityAlarm(ctx, &stCopy)
	}
}

func (e *engine) RecordConnect(ctx context.Context, deviceID ids.ID, ts int64) {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	e.mu.Lock()
	st, ok := e.states[deviceID]
	if !ok {
		st = &State{
			DeviceID:            deviceID,
			InactivityTimeoutMs: e.defaultTimeoutMs,
		}
		e.states[deviceID] = st
	}

	st.LastConnectTime = ts
	if ts > st.LastActivityTime {
		st.LastActivityTime = ts
	}
	st.Active = true
	st.Deadline = ts + st.InactivityTimeoutMs
	stCopy := *st
	e.mu.Unlock()

	e.notifyListeners(ctx, EventConnect, &stCopy)
	e.persistAttributes(ctx, &stCopy)
	e.clearInactivityAlarm(ctx, &stCopy)
}

func (e *engine) RecordDisconnect(ctx context.Context, deviceID ids.ID, ts int64) {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	e.mu.Lock()
	st, ok := e.states[deviceID]
	if !ok {
		st = &State{
			DeviceID:            deviceID,
			InactivityTimeoutMs: e.defaultTimeoutMs,
		}
		e.states[deviceID] = st
	}

	st.LastDisconnectTime = ts
	stCopy := *st
	e.mu.Unlock()

	e.notifyListeners(ctx, EventDisconnect, &stCopy)
	e.persistAttributes(ctx, &stCopy)
}

func (e *engine) IsActive(ctx context.Context, deviceID ids.ID) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	st, ok := e.states[deviceID]
	if !ok {
		return false
	}
	if st.Overridden {
		return st.Active
	}
	if !st.Active {
		return false
	}
	return time.Now().UnixMilli() < st.Deadline
}

func (e *engine) GetActivity(ctx context.Context, deviceID ids.ID) model.DeviceActivity {
	e.mu.RLock()
	defer e.mu.RUnlock()

	st, ok := e.states[deviceID]
	if !ok {
		return model.DeviceActivity{
			DeviceID: deviceID,
			Active:   false,
		}
	}

	active := st.Active
	if !st.Overridden && active {
		active = time.Now().UnixMilli() < st.Deadline
	}

	return model.DeviceActivity{
		DeviceID:            deviceID,
		LastActivityTime:    st.LastActivityTime,
		Active:              active,
		LastConnectTime:     st.LastConnectTime,
		LastDisconnectTime:  st.LastDisconnectTime,
		InactivityTimeoutMs: st.InactivityTimeoutMs,
	}
}

func (e *engine) GetState(ctx context.Context, deviceID ids.ID) (*State, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	st, ok := e.states[deviceID]
	if !ok {
		return nil, errs.NotFound("device state not found")
	}

	res := *st
	if !st.Overridden && res.Active {
		res.Active = time.Now().UnixMilli() < res.Deadline
	}
	return &res, nil
}

func (e *engine) SetOverride(ctx context.Context, deviceID ids.ID, active bool) (*State, error) {
	e.mu.Lock()
	st, ok := e.states[deviceID]
	if !ok {
		st = &State{
			DeviceID:            deviceID,
			InactivityTimeoutMs: e.defaultTimeoutMs,
		}
		e.states[deviceID] = st
	}

	st.Active = active
	st.Overridden = true
	if active {
		st.Deadline = time.Now().UnixMilli() + st.InactivityTimeoutMs
	}
	stCopy := *st
	e.mu.Unlock()

	e.persistAttributes(ctx, &stCopy)
	return &stCopy, nil
}

func (e *engine) GetSummary(ctx context.Context, tenantID ids.ID) (*StateSummary, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now().UnixMilli()
	summary := &StateSummary{TenantID: tenantID}

	for _, st := range e.states {
		if tenantID != ids.Nil && st.TenantID != ids.Nil && st.TenantID != tenantID {
			continue
		}
		summary.Total++
		active := st.Active
		if !st.Overridden && active {
			active = now < st.Deadline
		}
		if active {
			summary.Active++
		} else {
			summary.Inactive++
		}
	}
	return summary, nil
}

func (e *engine) persistAttributes(ctx context.Context, st *State) {
	if e.attrStore == nil {
		return
	}

	attrs := map[string]any{
		"active":             st.Active,
		"lastActivityTime":   st.LastActivityTime,
		"lastConnectTime":    st.LastConnectTime,
		"lastDisconnectTime": st.LastDisconnectTime,
		"inactivityTimeout":  st.InactivityTimeoutMs,
	}
	if st.InactivityAlarmTime > 0 {
		attrs["inactivityAlarmTime"] = st.InactivityAlarmTime
	}

	_ = e.attrStore.SaveAttributes(ctx, st.DeviceID, model.ScopeServer, attrs)
}

func (e *engine) raiseInactivityAlarm(ctx context.Context, st *State) {
	if e.alarmSvc == nil || st.TenantID == ids.Nil {
		return
	}

	detailsJSON, _ := json.Marshal(map[string]any{
		"message":               fmt.Sprintf("Device %s is inactive (offline)", st.DeviceID),
		"last_activity_time":    st.LastActivityTime,
		"inactivity_timeout_ms": st.InactivityTimeoutMs,
	})

	now := time.Now().UnixMilli()
	al := &model.Alarm{
		ID:           ids.MustNew(),
		CreatedTime:  now,
		TenantID:     st.TenantID,
		OriginatorID: st.DeviceID,
		Type:         "Inactivity",
		Severity:     model.SeverityMajor,
		Status:       model.AlarmActiveUnack,
		StartTs:      now,
		Details:      detailsJSON,
	}
	_, _ = e.alarmSvc.CreateOrUpdateAlarm(ctx, al)
}

func (e *engine) clearInactivityAlarm(ctx context.Context, st *State) {
	if e.alarmSvc == nil || st.TenantID == ids.Nil {
		return
	}

	alarms, _, err := e.alarmSvc.ListAlarms(ctx, st.TenantID, &st.DeviceID, 0, 50)
	if err != nil {
		return
	}

	for _, a := range alarms {
		if a.Type == "Inactivity" &&
			(a.Status == model.AlarmActiveUnack || a.Status == model.AlarmActiveAck) {
			_, _ = e.alarmSvc.ClearAlarm(ctx, st.TenantID, a.ID)
		}
	}
}
