package dashboard

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

// Service defines dashboard management operations.
type Service interface {
	CreateDashboard(ctx context.Context, tenantID ids.ID, title string, config json.RawMessage) (*model.Dashboard, error)
	GetDashboard(ctx context.Context, tenantID, id ids.ID) (*model.Dashboard, error)
	UpdateDashboard(ctx context.Context, tenantID, id ids.ID, title string, config json.RawMessage) (*model.Dashboard, error)
	DeleteDashboard(ctx context.Context, tenantID, id ids.ID) error
	ListDashboards(ctx context.Context, tenantID ids.ID, page, pageSize int) ([]model.Dashboard, int, error)
}

type memoryService struct {
	mu         sync.RWMutex
	dashboards map[string]*model.Dashboard
}

// NewMemoryService returns an in-memory Dashboard service.
func NewMemoryService() Service {
	return &memoryService{
		dashboards: make(map[string]*model.Dashboard),
	}
}

func (s *memoryService) CreateDashboard(ctx context.Context, tenantID ids.ID, title string, config json.RawMessage) (*model.Dashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if title == "" {
		return nil, errs.InvalidArgument("dashboard title cannot be empty")
	}

	if len(config) == 0 {
		config = json.RawMessage(`{"widgets":[]}`)
	}

	d := &model.Dashboard{
		ID:            ids.MustNew(),
		CreatedTime:   time.Now().UnixMilli(),
		TenantID:      tenantID,
		Title:         title,
		Configuration: config,
	}

	s.dashboards[d.ID.String()] = d
	return d, nil
}

func (s *memoryService) GetDashboard(ctx context.Context, tenantID, id ids.ID) (*model.Dashboard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.dashboards[id.String()]
	if !ok || d.TenantID != tenantID {
		return nil, errs.NotFound("dashboard not found")
	}
	return d, nil
}

func (s *memoryService) UpdateDashboard(ctx context.Context, tenantID, id ids.ID, title string, config json.RawMessage) (*model.Dashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.dashboards[id.String()]
	if !ok || d.TenantID != tenantID {
		return nil, errs.NotFound("dashboard not found")
	}

	if title != "" {
		d.Title = title
	}
	if len(config) > 0 {
		d.Configuration = config
	}

	return d, nil
}

func (s *memoryService) DeleteDashboard(ctx context.Context, tenantID, id ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.dashboards[id.String()]
	if !ok || d.TenantID != tenantID {
		return errs.NotFound("dashboard not found")
	}

	delete(s.dashboards, id.String())
	return nil
}

func (s *memoryService) ListDashboards(ctx context.Context, tenantID ids.ID, page, pageSize int) ([]model.Dashboard, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	var list []model.Dashboard
	for _, d := range s.dashboards {
		if d.TenantID == tenantID {
			list = append(list, *d)
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedTime > list[j].CreatedTime
	})

	total := len(list)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []model.Dashboard{}, total, nil
	}

	end := offset + pageSize
	if end > total {
		end = total
	}

	return list[offset:end], total, nil
}
