package alarm

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Service defines the interface for Alarm lifecycle operations.
type Service interface {
	CreateOrUpdateAlarm(ctx context.Context, alarm *model.Alarm) (*model.Alarm, error)
	AckAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error)
	ClearAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error)
	GetAlarm(ctx context.Context, tenantID, alarmID ids.ID) (*model.Alarm, error)
	ListAlarms(ctx context.Context, tenantID ids.ID, originatorID *ids.ID, page, pageSize int) ([]model.Alarm, int, error)
}

type memoryService struct {
	mu     sync.RWMutex
	alarms map[string]*model.Alarm
}

// NewMemoryService returns an in-memory alarm management service.
func NewMemoryService() Service {
	return &memoryService{
		alarms: make(map[string]*model.Alarm),
	}
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
	if alarm.Status == model.AlarmActiveUnack {
		alarm.Status = model.AlarmActiveAck
	} else if alarm.Status == model.AlarmClearedUnack {
		alarm.Status = model.AlarmClearedAck
	}

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
	if alarm.Status == model.AlarmActiveUnack {
		alarm.Status = model.AlarmClearedUnack
	} else if alarm.Status == model.AlarmActiveAck {
		alarm.Status = model.AlarmClearedAck
	}

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
