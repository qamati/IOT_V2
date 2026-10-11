package scada

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Service provides industrial tag processing, ISA-18.2 alarm management, and SBO control.
type Service interface {
	RegisterTag(ctx context.Context, tag *Tag) error
	GetTag(ctx context.Context, tenantID, tagID ids.ID) (*Tag, error)
	GetTagByPath(ctx context.Context, tenantID ids.ID, path string) (*Tag, error)
	ListTags(ctx context.Context, tenantID ids.ID) ([]*Tag, error)
	UpdateTagValue(ctx context.Context, tenantID ids.ID, path string, rawValue any, quality string) (*Tag, bool, error)

	ListAlarms(ctx context.Context, tenantID ids.ID) ([]*ISA182Alarm, error)
	AcknowledgeAlarm(ctx context.Context, tenantID, alarmID ids.ID, user string) (*ISA182Alarm, error)
	ShelveAlarm(ctx context.Context, tenantID, alarmID ids.ID, durationMs int64) (*ISA182Alarm, error)

	SelectControl(ctx context.Context, tenantID ids.ID, path string, desiredValue any, operator string) (*ControlCommand, error)
	OperateControl(ctx context.Context, tenantID ids.ID, selectID ids.ID) (*ControlCommand, error)
}

type memoryService struct {
	mu       sync.RWMutex
	tags     map[ids.ID]*Tag
	byPath   map[string]ids.ID // tenantID:path -> tagID
	alarms   map[ids.ID]*ISA182Alarm
	controls map[ids.ID]*ControlCommand
}

func NewMemoryService() Service {
	s := &memoryService{
		tags:     make(map[ids.ID]*Tag),
		byPath:   make(map[string]ids.ID),
		alarms:   make(map[ids.ID]*ISA182Alarm),
		controls: make(map[ids.ID]*ControlCommand),
	}

	s.seedDefaultTags()
	return s
}

func (s *memoryService) seedDefaultTags() {
	now := time.Now().UnixMilli()

	tags := []*Tag{
		{
			ID:          ids.MustNew(),
			TenantID:    ids.Nil,
			Path:        "WaterTreatment.Tank101.Level",
			DataType:    DataTypeAnalog,
			EngUnit:     "%",
			Deadband:    0.5,
			ScaleFactor: 1.0,
			EUMin:       0.0,
			EUMax:       100.0,
			ScanRateMs:  1000,
			Value:       76.4,
			Quality:     QualityGood,
			Timestamp:   now,
			Version:     1,
		},
		{
			ID:          ids.MustNew(),
			TenantID:    ids.Nil,
			Path:        "WaterTreatment.Pump101.Status",
			DataType:    DataTypeDiscrete,
			EngUnit:     "STATE",
			ScanRateMs:  500,
			Value:       1,
			Quality:     QualityGood,
			Timestamp:   now,
			Version:     1,
		},
		{
			ID:          ids.MustNew(),
			TenantID:    ids.Nil,
			Path:        "WaterTreatment.Pump101.Flow",
			DataType:    DataTypeAnalog,
			EngUnit:     "GPM",
			Deadband:    1.0,
			ScaleFactor: 1.0,
			EUMin:       0.0,
			EUMax:       500.0,
			ScanRateMs:  1000,
			Value:       342.8,
			Quality:     QualityGood,
			Timestamp:   now,
			Version:     1,
		},
		{
			ID:          ids.MustNew(),
			TenantID:    ids.Nil,
			Path:        "WaterTreatment.Discharge.Pressure",
			DataType:    DataTypeAnalog,
			EngUnit:     "PSI",
			Deadband:    0.2,
			ScaleFactor: 1.0,
			EUMin:       0.0,
			EUMax:       150.0,
			ScanRateMs:  1000,
			Value:       62.5,
			Quality:     QualityGood,
			Timestamp:   now,
			Version:     1,
		},
	}

	for _, t := range tags {
		s.tags[t.ID] = t
		key := fmt.Sprintf("%s:%s", t.TenantID.String(), t.Path)
		s.byPath[key] = t.ID
	}
}

func (s *memoryService) RegisterTag(ctx context.Context, tag *Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tag.ID == ids.Nil {
		tag.ID = ids.MustNew()
	}
	if tag.Quality == "" {
		tag.Quality = QualityGood
	}
	if tag.Timestamp == 0 {
		tag.Timestamp = time.Now().UnixMilli()
	}
	tag.Version = 1

	s.tags[tag.ID] = tag
	key := fmt.Sprintf("%s:%s", tag.TenantID.String(), tag.Path)
	s.byPath[key] = tag.ID
	return nil
}

func (s *memoryService) GetTag(ctx context.Context, tenantID, tagID ids.ID) (*Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tags[tagID]
	if !ok || (t.TenantID != ids.Nil && t.TenantID != tenantID) {
		return nil, errs.NotFound("tag not found")
	}
	return t, nil
}

func (s *memoryService) GetTagByPath(ctx context.Context, tenantID ids.ID, path string) (*Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", tenantID.String(), path)
	id, ok := s.byPath[key]
	if !ok {
		// Also check system tenant nil
		keyNil := fmt.Sprintf("%s:%s", ids.Nil.String(), path)
		id, ok = s.byPath[keyNil]
		if !ok {
			return nil, errs.NotFound(fmt.Sprintf("tag '%s' not found", path))
		}
	}
	return s.tags[id], nil
}

func (s *memoryService) ListTags(ctx context.Context, tenantID ids.ID) ([]*Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Tag
	for _, t := range s.tags {
		if t.TenantID == ids.Nil || t.TenantID == tenantID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (s *memoryService) UpdateTagValue(ctx context.Context, tenantID ids.ID, path string, rawValue any, quality string) (*Tag, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", tenantID.String(), path)
	id, ok := s.byPath[key]
	if !ok {
		keyNil := fmt.Sprintf("%s:%s", ids.Nil.String(), path)
		id, ok = s.byPath[keyNil]
		if !ok {
			return nil, false, errs.NotFound("tag path not found")
		}
	}

	tag := s.tags[id]

	// Apply scaling if analog
	scaledVal := rawValue
	if tag.DataType == DataTypeAnalog {
		if num, ok := toFloat64(rawValue); ok {
			scale := tag.ScaleFactor
			if scale == 0 {
				scale = 1.0
			}
			computed := num*scale + tag.ScaleOffset
			scaledVal = computed

			// Check Deadband
			if tag.Deadband > 0 && tag.Value != nil {
				if lastNum, ok := toFloat64(tag.Value); ok {
					diff := math.Abs(computed - lastNum)
					if diff < tag.Deadband && quality == tag.Quality {
						// Inside deadband: refresh timestamp only, suppress update propagation
						tag.Timestamp = time.Now().UnixMilli()
						return tag, false, nil
					}
				}
			}
		}
	}

	tag.LastValue = tag.Value
	tag.Value = scaledVal
	if quality != "" {
		tag.Quality = quality
	}
	tag.Timestamp = time.Now().UnixMilli()
	tag.Version++

	// Check Alarm Thresholds (e.g. Tank Level > 90% HIGH_HIGH)
	if tag.DataType == DataTypeAnalog {
		if num, ok := toFloat64(scaledVal); ok && num > 90.0 {
			s.triggerISA182Alarm(tenantID, tag.Path, "HIGH_HIGH", num, 90.0, "Critical Tank High High Level Exceeded")
		}
	}

	return tag, true, nil
}

func (s *memoryService) triggerISA182Alarm(tenantID ids.ID, path, alarmType string, currentVal, setpoint float64, msg string) {
	// Look for existing alarm on this tag
	for _, a := range s.alarms {
		if a.TagPath == path && a.AlarmType == alarmType {
			if a.State == AlarmStateShelved && time.Now().UnixMilli() < a.ShelveUntil {
				return // Still shelved
			}
			if a.State == AlarmStateNormal {
				a.State = AlarmStateUnackAlarm
				a.ActiveTs = time.Now().UnixMilli()
			}
			return
		}
	}

	alarmID := ids.MustNew()
	s.alarms[alarmID] = &ISA182Alarm{
		ID:        alarmID,
		TenantID:  tenantID,
		TagPath:   path,
		AlarmType: alarmType,
		Setpoint:  setpoint,
		Severity:  "CRITICAL",
		State:     AlarmStateUnackAlarm,
		ActiveTs:  time.Now().UnixMilli(),
		Message:   msg,
	}
}

func (s *memoryService) ListAlarms(ctx context.Context, tenantID ids.ID) ([]*ISA182Alarm, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ISA182Alarm
	for _, a := range s.alarms {
		if a.TenantID == ids.Nil || a.TenantID == tenantID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (s *memoryService) AcknowledgeAlarm(ctx context.Context, tenantID, alarmID ids.ID, user string) (*ISA182Alarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.alarms[alarmID]
	if !ok || (a.TenantID != ids.Nil && a.TenantID != tenantID) {
		return nil, errs.NotFound("alarm not found")
	}

	if a.State == AlarmStateUnackAlarm {
		a.State = AlarmStateAckedAlarm
		a.AckTs = time.Now().UnixMilli()
		a.AckUser = user
	}
	return a, nil
}

func (s *memoryService) ShelveAlarm(ctx context.Context, tenantID, alarmID ids.ID, durationMs int64) (*ISA182Alarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.alarms[alarmID]
	if !ok || (a.TenantID != ids.Nil && a.TenantID != tenantID) {
		return nil, errs.NotFound("alarm not found")
	}

	if durationMs <= 0 {
		durationMs = 300000 // 5 minutes default
	}

	a.State = AlarmStateShelved
	a.ShelveUntil = time.Now().UnixMilli() + durationMs
	return a, nil
}

func (s *memoryService) SelectControl(ctx context.Context, tenantID ids.ID, path string, desiredValue any, operator string) (*ControlCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := &ControlCommand{
		SelectID:     ids.MustNew(),
		TenantID:     tenantID,
		TagPath:      path,
		DesiredValue: desiredValue,
		Operator:     operator,
		SelectedAt:   time.Now().UnixMilli(),
		Status:       "SELECTED",
	}

	s.controls[cmd.SelectID] = cmd
	return cmd, nil
}

func (s *memoryService) OperateControl(ctx context.Context, tenantID ids.ID, selectID ids.ID) (*ControlCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd, ok := s.controls[selectID]
	if !ok || (cmd.TenantID != ids.Nil && cmd.TenantID != tenantID) {
		return nil, errs.NotFound("control select command not found")
	}

	if cmd.Status != "SELECTED" {
		return nil, errs.InvalidArgument(fmt.Sprintf("command cannot be operated from status '%s'", cmd.Status))
	}

	// Verify expiration (SBO selection timeout is 60s)
	if time.Now().UnixMilli()-cmd.SelectedAt > 60000 {
		cmd.Status = "CANCELLED"
		cmd.Reason = "SBO selection expired"
		return cmd, errs.InvalidArgument("Select-Before-Operate window expired")
	}

	// Apply value to tag
	key := fmt.Sprintf("%s:%s", tenantID.String(), cmd.TagPath)
	id, ok := s.byPath[key]
	if !ok {
		keyNil := fmt.Sprintf("%s:%s", ids.Nil.String(), cmd.TagPath)
		id = s.byPath[keyNil]
	}

	if tag, exists := s.tags[id]; exists {
		tag.LastValue = tag.Value
		tag.Value = cmd.DesiredValue
		tag.Timestamp = time.Now().UnixMilli()
		tag.Version++
	}

	cmd.Status = "OPERATED"
	cmd.OperatedAt = time.Now().UnixMilli()
	return cmd, nil
}

func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	return 0, false
}
