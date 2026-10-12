package alarm

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// TopicAlarmEvents is the standard NATS/JetStream topic for alarm lifecycle events.
const TopicAlarmEvents = "alarm.events"

// Service defines the interface for Alarm lifecycle and Alarm Rules operations.
type Service interface {
	CreateOrUpdateAlarm(ctx context.Context, alarm *model.Alarm) (*model.Alarm, error)
	AckAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error)
	ClearAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error)
	GetAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error)
	ListAlarms(ctx context.Context, tenantID ids.ID, originatorID *ids.ID, page, pageSize int) ([]model.Alarm, int, error)

	// Stage 5 Alarm Rules & Evaluator
	SaveAlarmRule(ctx context.Context, rule *model.AlarmRule) (*model.AlarmRule, error)
	GetAlarmRule(ctx context.Context, tenantID, ruleID ids.ID) (*model.AlarmRule, error)
	ListAlarmRules(ctx context.Context, tenantID ids.ID) ([]model.AlarmRule, error)
	DeleteAlarmRule(ctx context.Context, tenantID, ruleID ids.ID) error
	TestAlarmRule(ctx context.Context, req *model.AlarmRuleTestRequest) (*model.AlarmRuleTestResponse, error)
	EvaluateTelemetry(ctx context.Context, tenantID, deviceID ids.ID, ts int64, values map[string]any) ([]*model.Alarm, error)
	SetEventBus(b bus.Bus)
}

type memoryService struct {
	mu        sync.RWMutex
	alarms    map[string]*model.Alarm
	rules     map[string]*model.AlarmRule
	evaluator *Evaluator
	bus       bus.Bus
}

// NewMemoryService returns an in-memory alarm management service.
func NewMemoryService() Service {
	return &memoryService{
		alarms:    make(map[string]*model.Alarm),
		rules:     make(map[string]*model.AlarmRule),
		evaluator: NewEvaluator(),
	}
}

func (s *memoryService) SetEventBus(b bus.Bus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bus = b
}

func (s *memoryService) publishAlarmEvent(ctx context.Context, eventType string, a *model.Alarm) {
	if s.bus == nil || a == nil {
		return
	}
	payload, _ := json.Marshal(a)
	msg := bus.NewMessage(a.TenantID.String(), "DEVICE", a.OriginatorID.String(), eventType, payload)
	_ = s.bus.Publish(ctx, TopicAlarmEvents, msg)
}

func (s *memoryService) CreateOrUpdateAlarm(ctx context.Context, in *model.Alarm) (*model.Alarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	// Check if an existing un-cleared alarm of the same type and originator exists
	for _, existing := range s.alarms {
		if existing.TenantID == in.TenantID &&
			existing.OriginatorID == in.OriginatorID &&
			existing.Type == in.Type &&
			(existing.Status == model.AlarmActiveUnack || existing.Status == model.AlarmActiveAck) {

			// Deduplicate: update existing alarm
			existing.EndTs = now
			existing.Severity = in.Severity
			if len(in.Details) > 0 {
				existing.Details = in.Details
			}
			go s.publishAlarmEvent(context.Background(), bus.TypeAlarmCreated, existing)
			return existing, nil
		}
	}

	// Create new alarm
	if in.ID == ids.Nil {
		in.ID = ids.MustNew()
	}
	if in.CreatedTime <= 0 {
		in.CreatedTime = now
	}
	if in.StartTs <= 0 {
		in.StartTs = now
	}
	if in.EndTs <= 0 {
		in.EndTs = now
	}
	if in.Status == "" {
		in.Status = model.AlarmActiveUnack
	}

	stored := *in
	s.alarms[stored.ID.String()] = &stored
	go s.publishAlarmEvent(context.Background(), bus.TypeAlarmCreated, &stored)
	return &stored, nil
}

func (s *memoryService) AckAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alarm, ok := s.alarms[alarmID.String()]
	if !ok || alarm.TenantID != tenantID {
		return nil, errs.NotFound("alarm not found")
	}

	alarm.AckTs = time.Now().UnixMilli()
	switch alarm.Status {
	case model.AlarmActiveUnack:
		alarm.Status = model.AlarmActiveAck
	case model.AlarmClearedUnack:
		alarm.Status = model.AlarmClearedAck
	}

	go s.publishAlarmEvent(context.Background(), "ALARM_ACKED", alarm)
	return alarm, nil
}

func (s *memoryService) ClearAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alarm, ok := s.alarms[alarmID.String()]
	if !ok || alarm.TenantID != tenantID {
		return nil, errs.NotFound("alarm not found")
	}

	alarm.ClearTs = time.Now().UnixMilli()
	switch alarm.Status {
	case model.AlarmActiveUnack:
		alarm.Status = model.AlarmClearedUnack
	case model.AlarmActiveAck:
		alarm.Status = model.AlarmClearedAck
	}

	go s.publishAlarmEvent(context.Background(), bus.TypeAlarmCleared, alarm)
	return alarm, nil
}

func (s *memoryService) GetAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alarm, ok := s.alarms[alarmID.String()]
	if !ok || alarm.TenantID != tenantID {
		return nil, errs.NotFound("alarm not found")
	}
	return alarm, nil
}

func (s *memoryService) ListAlarms(ctx context.Context, tenantID ids.ID, originatorID *ids.ID, page, pageSize int) ([]model.Alarm, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	var matched []model.Alarm
	for _, a := range s.alarms {
		if a.TenantID != tenantID {
			continue
		}
		if originatorID != nil && a.OriginatorID != *originatorID {
			continue
		}
		matched = append(matched, *a)
	}

	// Sort descending by CreatedTime
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedTime > matched[j].CreatedTime
	})

	total := len(matched)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []model.Alarm{}, total, nil
	}

	end := offset + pageSize
	if end > total {
		end = total
	}

	return matched[offset:end], total, nil
}

// Stage 5 Alarm Rules implementations

func (s *memoryService) SaveAlarmRule(ctx context.Context, rule *model.AlarmRule) (*model.AlarmRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rule.ID == ids.Nil {
		rule.ID = ids.MustNew()
	}
	if rule.CreatedTime == 0 {
		rule.CreatedTime = time.Now().UnixMilli()
	}
	if rule.Version == 0 {
		rule.Version = 1
	}

	saved := *rule
	s.rules[rule.ID.String()] = &saved
	return &saved, nil
}

func (s *memoryService) GetAlarmRule(ctx context.Context, tenantID, ruleID ids.ID) (*model.AlarmRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rule, ok := s.rules[ruleID.String()]
	if !ok || rule.TenantID != tenantID {
		return nil, errs.NotFound("alarm rule not found")
	}
	return rule, nil
}

func (s *memoryService) ListAlarmRules(ctx context.Context, tenantID ids.ID) ([]model.AlarmRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]model.AlarmRule, 0)
	for _, r := range s.rules {
		if r.TenantID == tenantID {
			res = append(res, *r)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedTime > res[j].CreatedTime
	})
	return res, nil
}

func (s *memoryService) DeleteAlarmRule(ctx context.Context, tenantID, ruleID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, ok := s.rules[ruleID.String()]
	if !ok || rule.TenantID != tenantID {
		return errs.NotFound("alarm rule not found")
	}
	delete(s.rules, ruleID.String())
	return nil
}

func (s *memoryService) TestAlarmRule(ctx context.Context, req *model.AlarmRuleTestRequest) (*model.AlarmRuleTestResponse, error) {
	return TestSimulation(req), nil
}

func (s *memoryService) EvaluateTelemetry(ctx context.Context, tenantID, deviceID ids.ID, ts int64, values map[string]any) ([]*model.Alarm, error) {
	s.mu.RLock()
	rules := make([]model.AlarmRule, 0)
	for _, r := range s.rules {
		if r.TenantID == tenantID && r.Enabled {
			rules = append(rules, *r)
		}
	}
	s.mu.RUnlock()

	if len(rules) == 0 {
		return nil, nil
	}

	severities := []model.AlarmSeverity{
		model.SeverityCritical,
		model.SeverityMajor,
		model.SeverityMinor,
		model.SeverityWarning,
		model.SeverityIndeterminate,
	}

	var triggeredAlarms []*model.Alarm

	for _, rule := range rules {
		matchedSeverity := model.AlarmSeverity("")
		var matchedReason string
		var detailsTemplate string

		for _, sev := range severities {
			createDef, exists := rule.CreateRules[sev]
			if !exists {
				continue
			}

			matched, reason, _, _ := s.evaluator.EvaluateCondition(deviceID, rule.ID, sev, createDef.Condition, ts, values)
			if matched {
				matchedSeverity = sev
				matchedReason = reason
				detailsTemplate = createDef.Details
				break
			}
		}

		if matchedSeverity != "" {
			// Trigger or Update Alarm
			detailsMap := map[string]any{
				"rule_id":   rule.ID.String(),
				"rule_name": rule.Name,
				"reason":    matchedReason,
				"values":    values,
			}
			if detailsTemplate != "" {
				detailsMap["template"] = detailsTemplate
			}
			detailsBytes, _ := json.Marshal(detailsMap)

			al := &model.Alarm{
				TenantID:     tenantID,
				OriginatorID: deviceID,
				Type:         rule.AlarmType,
				Severity:     matchedSeverity,
				StartTs:      ts,
				EndTs:        ts,
				Details:      detailsBytes,
			}
			saved, err := s.CreateOrUpdateAlarm(ctx, al)
			if err == nil && saved != nil {
				triggeredAlarms = append(triggeredAlarms, saved)
			}
		} else if rule.ClearRule != nil {
			// Test Clear Rule
			matched, _, _, _ := s.evaluator.EvaluateCondition(deviceID, rule.ID, "CLEAR", rule.ClearRule.Condition, ts, values)
			if matched {
				// Find active alarm for this originator and type
				s.mu.RLock()
				var activeAlarmID ids.ID
				for _, existing := range s.alarms {
					if existing.TenantID == tenantID &&
						existing.OriginatorID == deviceID &&
						existing.Type == rule.AlarmType &&
						(existing.Status == model.AlarmActiveUnack || existing.Status == model.AlarmActiveAck) {
						activeAlarmID = existing.ID
						break
					}
				}
				s.mu.RUnlock()

				if activeAlarmID != ids.Nil {
					cleared, err := s.ClearAlarm(ctx, tenantID, activeAlarmID)
					if err == nil && cleared != nil {
						triggeredAlarms = append(triggeredAlarms, cleared)
					}
				}
			}
		}
	}

	return triggeredAlarms, nil
}
