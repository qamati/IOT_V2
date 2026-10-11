package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/internal/telemetry"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Service manages converters, integrations, webhooks, and the identity resolution pipeline.
type Service interface {
	CreateConverter(ctx context.Context, c *Converter) error
	GetConverter(ctx context.Context, tenantID, converterID ids.ID) (*Converter, error)
	ListConverters(ctx context.Context, tenantID ids.ID) ([]*Converter, error)
	DeleteConverter(ctx context.Context, tenantID, converterID ids.ID) error

	CreateIntegration(ctx context.Context, i *Integration) error
	GetIntegration(ctx context.Context, tenantID, integrationID ids.ID) (*Integration, error)
	GetIntegrationByRoutingKey(ctx context.Context, routingKey string) (*Integration, error)
	ListIntegrations(ctx context.Context, tenantID ids.ID) ([]*Integration, error)
	DeleteIntegration(ctx context.Context, tenantID, integrationID ids.ID) error

	GetStatus(ctx context.Context, integrationID ids.ID) (*IntegrationStatus, error)
	GetDebugEvents(ctx context.Context, integrationID ids.ID) ([]*DebugEvent, error)

	TestConverter(ctx context.Context, conv *Converter, payload []byte, meta map[string]string) (*UplinkEvent, error)
	ProcessWebhook(ctx context.Context, routingKey string, payload []byte, headers map[string]string) (*UplinkEvent, error)
}

type memoryService struct {
	mu           sync.RWMutex
	converters   map[ids.ID]*Converter
	integrations map[ids.ID]*Integration
	byRoutingKey map[string]ids.ID
	statuses     map[ids.ID]*IntegrationStatus
	debugLogs    map[ids.ID][]*DebugEvent

	engine     *Engine
	devManager *core.DeviceManager
	telemetry  telemetry.Store
	attributes telemetry.AttributeStore
	tracker    devstate.Tracker
	ruleEngine *rule.Engine
}

func NewMemoryService(
	devMgr *core.DeviceManager,
	ts telemetry.Store,
	attrs telemetry.AttributeStore,
	tracker devstate.Tracker,
	ruleEng *rule.Engine,
) Service {
	s := &memoryService{
		converters:   make(map[ids.ID]*Converter),
		integrations: make(map[ids.ID]*Integration),
		byRoutingKey: make(map[string]ids.ID),
		statuses:     make(map[ids.ID]*IntegrationStatus),
		debugLogs:    make(map[ids.ID][]*DebugEvent),
		engine:       NewEngine(),
		devManager:   devMgr,
		telemetry:    ts,
		attributes:   attrs,
		tracker:      tracker,
		ruleEngine:   ruleEng,
	}

	s.seedDefaultConverters()
	return s
}

func (s *memoryService) seedDefaultConverters() {
	// Seed 1: JSON Telemetry Webhook Converter
	c1ID := ids.MustNew()
	def1, _ := json.Marshal(DeclarativeDefinition{
		Format:         "json",
		DeviceNamePath: "device_name",
		DeviceType:     "environmental_sensor",
		Mappings: []DeclarativeMapping{
			{Source: "data.temp", Target: "telemetry.temperature", Type: "float"},
			{Source: "data.humidity", Target: "telemetry.humidity", Type: "float"},
			{Source: "data.battery", Target: "attributes.battery_level", Type: "int"},
			{Source: "status.firmware", Target: "attributes.firmware_version", Type: "string"},
		},
	})
	s.converters[c1ID] = &Converter{
		ID:         c1ID,
		TenantID:   ids.ID{}, // System template
		Name:       "Default JSON Webhook Converter",
		Direction:  DirectionUplink,
		Mode:       ModeDeclarative,
		Definition: def1,
		Version:    1,
	}

	// Seed 2: LoRaWAN ChirpStack Converter
	c2ID := ids.MustNew()
	def2, _ := json.Marshal(DeclarativeDefinition{
		Format:         "chirpstack",
		DeviceNamePath: "deviceInfo.deviceName",
		DeviceType:     "lorawan_node",
	})
	s.converters[c2ID] = &Converter{
		ID:         c2ID,
		TenantID:   ids.ID{},
		Name:       "ChirpStack LoRaWAN Converter",
		Direction:  DirectionUplink,
		Mode:       ModeDeclarative,
		Definition: def2,
		Version:    1,
	}
}

func (s *memoryService) CreateConverter(ctx context.Context, c *Converter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c.ID == ids.Nil {
		c.ID = ids.MustNew()
	}
	if c.Version == 0 {
		c.Version = 1
	}
	s.converters[c.ID] = c
	return nil
}

func (s *memoryService) GetConverter(ctx context.Context, tenantID, converterID ids.ID) (*Converter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.converters[converterID]
	if !ok || (c.TenantID != ids.Nil && c.TenantID != tenantID) {
		return nil, errs.NotFound("converter not found")
	}
	return c, nil
}

func (s *memoryService) ListConverters(ctx context.Context, tenantID ids.ID) ([]*Converter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Converter
	for _, c := range s.converters {
		if c.TenantID == ids.Nil || c.TenantID == tenantID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (s *memoryService) DeleteConverter(ctx context.Context, tenantID, converterID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.converters[converterID]
	if !ok || (c.TenantID != ids.Nil && c.TenantID != tenantID) {
		return errs.NotFound("converter not found")
	}
	delete(s.converters, converterID)
	return nil
}

func (s *memoryService) CreateIntegration(ctx context.Context, i *Integration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i.ID == ids.Nil {
		i.ID = ids.MustNew()
	}
	if i.RoutingKey == "" {
		i.RoutingKey = fmt.Sprintf("wh_%s", i.ID.String()[:8])
	}
	if i.Version == 0 {
		i.Version = 1
	}

	s.integrations[i.ID] = i
	s.byRoutingKey[i.RoutingKey] = i.ID
	s.statuses[i.ID] = &IntegrationStatus{
		IntegrationID: i.ID,
		State:         "RUNNING",
		LastOkTs:      time.Now().UnixMilli(),
		Counters:      map[string]int64{"rx_events": 0, "tx_events": 0, "errors": 0, "auto_created": 0},
		Instance:      "standalone-lite",
	}
	return nil
}

func (s *memoryService) GetIntegration(ctx context.Context, tenantID, integrationID ids.ID) (*Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	i, ok := s.integrations[integrationID]
	if !ok || i.TenantID != tenantID {
		return nil, errs.NotFound("integration not found")
	}
	return i, nil
}

func (s *memoryService) GetIntegrationByRoutingKey(ctx context.Context, routingKey string) (*Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byRoutingKey[routingKey]
	if !ok {
		return nil, errs.NotFound("integration routing key not found")
	}
	return s.integrations[id], nil
}

func (s *memoryService) ListIntegrations(ctx context.Context, tenantID ids.ID) ([]*Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Integration
	for _, i := range s.integrations {
		if i.TenantID == tenantID {
			result = append(result, i)
		}
	}
	return result, nil
}

func (s *memoryService) DeleteIntegration(ctx context.Context, tenantID, integrationID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	i, ok := s.integrations[integrationID]
	if !ok || i.TenantID != tenantID {
		return errs.NotFound("integration not found")
	}
	delete(s.byRoutingKey, i.RoutingKey)
	delete(s.integrations, integrationID)
	delete(s.statuses, integrationID)
	delete(s.debugLogs, integrationID)
	return nil
}

func (s *memoryService) GetStatus(ctx context.Context, integrationID ids.ID) (*IntegrationStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, ok := s.statuses[integrationID]
	if !ok {
		return nil, errs.NotFound("status not found")
	}
	return st, nil
}

func (s *memoryService) GetDebugEvents(ctx context.Context, integrationID ids.ID) ([]*DebugEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.debugLogs[integrationID], nil
}

func (s *memoryService) TestConverter(ctx context.Context, conv *Converter, payload []byte, meta map[string]string) (*UplinkEvent, error) {
	return s.engine.Convert(conv, payload, meta)
}

func (s *memoryService) ProcessWebhook(ctx context.Context, routingKey string, payload []byte, headers map[string]string) (*UplinkEvent, error) {
	integ, err := s.GetIntegrationByRoutingKey(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	if !integ.Enabled {
		return nil, errs.InvalidArgument("integration is disabled")
	}

	// Lookup uplink converter
	var conv *Converter
	s.mu.RLock()
	conv = s.converters[integ.UplinkConverterID]
	s.mu.RUnlock()

	if conv == nil {
		s.recordError(integ.ID, "configured uplink converter not found")
		return nil, errs.InvalidArgument("configured uplink converter not found")
	}

	// Run conversion
	event, err := s.engine.Convert(conv, payload, headers)
	if err != nil {
		s.recordError(integ.ID, err.Error())
		s.recordDebug(integ.ID, DirectionUplink, string(payload), headers, false, err.Error(), nil)
		return nil, fmt.Errorf("converter failed: %w", err)
	}

	// Identity Resolution: find or auto-create device
	deviceID, autoCreated, err := s.resolveIdentity(ctx, integ.TenantID, event.Device.Name, event.Device.Label, integ.AllowCreateDevice)
	if err != nil {
		s.recordError(integ.ID, "identity resolution failed: "+err.Error())
		return nil, err
	}

	// Save Telemetry Points
	var points []*model.TelemetryDataPoint
	for _, batch := range event.Telemetry {
		ts := batch.Ts
		if ts == 0 {
			ts = time.Now().UnixMilli()
		}
		for k, v := range batch.Values {
			points = append(points, createPoint(k, ts, v))
		}
	}

	if len(points) > 0 && s.telemetry != nil {
		_ = s.telemetry.Save(ctx, deviceID, points)
	}

	// Save Attributes
	if len(event.Attributes) > 0 && s.attributes != nil {
		_ = s.attributes.SaveAttributes(ctx, deviceID, model.ScopeClient, event.Attributes)
	}
	if len(event.Server) > 0 && s.attributes != nil {
		_ = s.attributes.SaveAttributes(ctx, deviceID, model.ScopeServer, event.Server)
	}

	// Notify Tracker & Rule Engine
	if s.tracker != nil {
		s.tracker.RecordActivity(ctx, deviceID, time.Now().UnixMilli())
	}
	if s.ruleEngine != nil && len(points) > 0 {
		s.ruleEngine.EvaluateTelemetry(ctx, integ.TenantID, deviceID, points)
	}

	// Update statistics & record debug log
	s.recordSuccess(integ.ID, autoCreated)
	s.recordDebug(integ.ID, DirectionUplink, string(payload), headers, true, "", event)

	return event, nil
}

func (s *memoryService) resolveIdentity(ctx context.Context, tenantID ids.ID, name, label string, allowCreate bool) (ids.ID, bool, error) {
	if s.devManager == nil {
		return ids.MustNew(), false, nil
	}

	// Check if device exists
	devices, _, err := s.devManager.ListDevices(ctx, tenantID, 0, 500)
	if err == nil {
		for _, d := range devices {
			if d.Name == name {
				return d.ID, false, nil
			}
		}
	}

	if !allowCreate {
		return ids.ID{}, false, errs.NotFound(fmt.Sprintf("device '%s' not registered and auto-provisioning disabled", name))
	}

	// Auto-provision
	dev, _, err := s.devManager.CreateDevice(ctx, tenantID, name, label, "")
	if err != nil {
		return ids.ID{}, false, fmt.Errorf("auto-provision device failed: %w", err)
	}
	return dev.ID, true, nil
}

func (s *memoryService) recordSuccess(id ids.ID, autoCreated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if st, ok := s.statuses[id]; ok {
		st.State = "RUNNING"
		st.LastOkTs = time.Now().UnixMilli()
		st.LastError = ""
		st.Counters["rx_events"]++
		if autoCreated {
			st.Counters["auto_created"]++
		}
	}
}

func (s *memoryService) recordError(id ids.ID, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if st, ok := s.statuses[id]; ok {
		st.LastError = errMsg
		st.Counters["errors"]++
	}
}

func (s *memoryService) recordDebug(id ids.ID, dir, payload string, meta map[string]string, success bool, errMsg string, result *UplinkEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	evt := &DebugEvent{
		ID:        ids.MustNew(),
		Ts:        time.Now().UnixMilli(),
		Direction: dir,
		Payload:   payload,
		Metadata:  meta,
		Success:   success,
		Error:     errMsg,
		Result:    result,
	}

	logs := s.debugLogs[id]
	if len(logs) >= 50 {
		logs = logs[1:]
	}
	s.debugLogs[id] = append(logs, evt)
}

func createPoint(key string, ts int64, val any) *model.TelemetryDataPoint {
	pt := &model.TelemetryDataPoint{
		Key:       key,
		Timestamp: ts,
	}
	switch v := val.(type) {
	case float64:
		pt.DblV = &v
	case float32:
		f := float64(v)
		pt.DblV = &f
	case int64:
		pt.LongV = &v
	case int:
		l := int64(v)
		pt.LongV = &l
	case bool:
		pt.BoolV = &v
	case string:
		pt.StrV = &v
	default:
		s := fmt.Sprint(v)
		pt.StrV = &s
	}
	return pt
}
