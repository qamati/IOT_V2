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

type defaultService struct {
	mu           sync.RWMutex
	store        Store
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

func NewService(
	store Store,
	devMgr *core.DeviceManager,
	ts telemetry.Store,
	attrs telemetry.AttributeStore,
	tracker devstate.Tracker,
	ruleEng *rule.Engine,
) Service {
	s := &defaultService{
		store:        store,
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

func NewMemoryService(
	devMgr *core.DeviceManager,
	ts telemetry.Store,
	attrs telemetry.AttributeStore,
	tracker devstate.Tracker,
	ruleEng *rule.Engine,
) Service {
	return NewService(nil, devMgr, ts, attrs, tracker, ruleEng)
}

func (s *defaultService) seedDefaultConverters() {
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
	conv1 := &Converter{
		ID:         c1ID,
		TenantID:   ids.ID{}, // System template
		Name:       "Default JSON Webhook Converter",
		Direction:  DirectionUplink,
		Mode:       ModeDeclarative,
		Definition: def1,
		Version:    1,
	}
	s.converters[c1ID] = conv1

	// Seed 2: LoRaWAN ChirpStack Converter
	c2ID := ids.MustNew()
	def2, _ := json.Marshal(DeclarativeDefinition{
		Format:         "chirpstack",
		DeviceNamePath: "deviceInfo.deviceName",
		DeviceType:     "lorawan_node",
	})
	conv2 := &Converter{
		ID:         c2ID,
		TenantID:   ids.ID{},
		Name:       "ChirpStack LoRaWAN Converter",
		Direction:  DirectionUplink,
		Mode:       ModeDeclarative,
		Definition: def2,
		Version:    1,
	}
	s.converters[c2ID] = conv2

	// Seed 3: The Things Network (TTN) Converter
	c3ID := ids.MustNew()
	def3, _ := json.Marshal(DeclarativeDefinition{
		Format:         "ttn",
		DeviceNamePath: "end_device_ids.device_id",
		DeviceType:     "lorawan_ttn_node",
	})
	conv3 := &Converter{
		ID:         c3ID,
		TenantID:   ids.ID{},
		Name:       "The Things Network (TTN) Converter",
		Direction:  DirectionUplink,
		Mode:       ModeDeclarative,
		Definition: def3,
		Version:    1,
	}
	s.converters[c3ID] = conv3

	if s.store != nil {
		ctx := context.Background()
		existing, _ := s.store.ListConverters(ctx, ids.ID{})
		if len(existing) == 0 {
			_ = s.store.CreateConverter(ctx, conv1)
			_ = s.store.CreateConverter(ctx, conv2)
			_ = s.store.CreateConverter(ctx, conv3)
		}
	}
}

func (s *defaultService) CreateConverter(ctx context.Context, c *Converter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c.ID == ids.Nil {
		c.ID = ids.MustNew()
	}
	if c.Version == 0 {
		c.Version = 1
	}

	if s.store != nil {
		if err := s.store.CreateConverter(ctx, c); err != nil {
			return err
		}
	}
	s.converters[c.ID] = c
	return nil
}

func (s *defaultService) GetConverter(ctx context.Context, tenantID, converterID ids.ID) (*Converter, error) {
	if s.store != nil {
		if c, err := s.store.GetConverter(ctx, tenantID, converterID); err == nil && c != nil {
			return c, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.converters[converterID]
	if !ok || (c.TenantID != ids.Nil && c.TenantID != tenantID) {
		return nil, errs.NotFound("converter not found")
	}
	return c, nil
}

func (s *defaultService) ListConverters(ctx context.Context, tenantID ids.ID) ([]*Converter, error) {
	if s.store != nil {
		convs, err := s.store.ListConverters(ctx, tenantID)
		if err == nil && len(convs) > 0 {
			return convs, nil
		}
	}

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

func (s *defaultService) DeleteConverter(ctx context.Context, tenantID, converterID ids.ID) error {
	if s.store != nil {
		_ = s.store.DeleteConverter(ctx, tenantID, converterID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.converters, converterID)
	return nil
}
func (s *defaultService) CreateIntegration(ctx context.Context, i *Integration) error {
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

	if s.store != nil {
		if err := s.store.CreateIntegration(ctx, i); err != nil {
			return err
		}
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

func (s *defaultService) GetIntegration(ctx context.Context, tenantID, integrationID ids.ID) (*Integration, error) {
	if s.store != nil {
		if i, err := s.store.GetIntegration(ctx, tenantID, integrationID); err == nil && i != nil {
			return i, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	i, ok := s.integrations[integrationID]
	if !ok || i.TenantID != tenantID {
		return nil, errs.NotFound("integration not found")
	}
	return i, nil
}

func (s *defaultService) GetIntegrationByRoutingKey(ctx context.Context, routingKey string) (*Integration, error) {
	if s.store != nil {
		if i, err := s.store.GetIntegrationByRoutingKey(ctx, routingKey); err == nil && i != nil {
			return i, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byRoutingKey[routingKey]
	if !ok {
		return nil, errs.NotFound("integration routing key not found")
	}
	return s.integrations[id], nil
}

func (s *defaultService) ListIntegrations(ctx context.Context, tenantID ids.ID) ([]*Integration, error) {
	if s.store != nil {
		integs, err := s.store.ListIntegrations(ctx, tenantID)
		if err == nil && len(integs) > 0 {
			return integs, nil
		}
	}

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

func (s *defaultService) DeleteIntegration(ctx context.Context, tenantID, integrationID ids.ID) error {
	if s.store != nil {
		_ = s.store.DeleteIntegration(ctx, tenantID, integrationID)
	}

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

func (s *defaultService) GetStatus(ctx context.Context, integrationID ids.ID) (*IntegrationStatus, error) {
	if s.store != nil {
		if st, err := s.store.GetStatus(ctx, integrationID); err == nil && st != nil {
			return st, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	st, ok := s.statuses[integrationID]
	if !ok {
		return nil, errs.NotFound("status not found")
	}
	return st, nil
}

func (s *defaultService) GetDebugEvents(ctx context.Context, integrationID ids.ID) ([]*DebugEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.debugLogs[integrationID], nil
}

func (s *defaultService) TestConverter(ctx context.Context, conv *Converter, payload []byte, meta map[string]string) (*UplinkEvent, error) {
	return s.engine.Convert(conv, payload, meta)
}

func (s *defaultService) ProcessWebhook(ctx context.Context, routingKey string, payload []byte, headers map[string]string) (*UplinkEvent, error) {
	integ, err := s.GetIntegrationByRoutingKey(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	if !integ.Enabled {
		return nil, errs.InvalidArgument("integration is disabled")
	}

	// Lookup uplink converter
	var conv *Converter
	if integ.UplinkConverterID != ids.Nil {
		conv, _ = s.GetConverter(ctx, integ.TenantID, integ.UplinkConverterID)
	}
	if conv == nil {
		s.mu.RLock()
		conv = s.converters[integ.UplinkConverterID]
		s.mu.RUnlock()
	}

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

func (s *defaultService) resolveIdentity(ctx context.Context, tenantID ids.ID, name, label string, allowCreate bool) (ids.ID, bool, error) {
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

func (s *defaultService) recordSuccess(id ids.ID, autoCreated bool) {
	s.mu.Lock()
	st, ok := s.statuses[id]
	if ok {
		st.State = "RUNNING"
		st.LastOkTs = time.Now().UnixMilli()
		st.LastError = ""
		st.Counters["rx_events"]++
		if autoCreated {
			st.Counters["auto_created"]++
		}
	}
	s.mu.Unlock()

	if ok && s.store != nil {
		_ = s.store.UpsertStatus(context.Background(), st)
	}
}

func (s *defaultService) recordError(id ids.ID, errMsg string) {
	s.mu.Lock()
	st, ok := s.statuses[id]
	if ok {
		st.LastError = errMsg
		st.Counters["errors"]++
	}
	s.mu.Unlock()

	if ok && s.store != nil {
		_ = s.store.UpsertStatus(context.Background(), st)
	}
}

func (s *defaultService) recordDebug(id ids.ID, dir, payload string, meta map[string]string, success bool, errMsg string, result *UplinkEvent) {
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
