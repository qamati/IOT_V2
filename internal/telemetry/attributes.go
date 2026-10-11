package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// AttributeStore provides CRUD operations for scoped entity attributes.
type AttributeStore interface {
	SaveAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, attrs map[string]any) error
	GetAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, keys []string) ([]model.AttributeKv, error)
	GetAllAttributes(ctx context.Context, entityID ids.ID) ([]model.AttributeKv, error)
	DeleteAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, keys []string) error
}

type memoryAttributeStore struct {
	mu   sync.RWMutex
	data map[string]map[model.AttributeScope]map[string]model.AttributeKv
}

// NewMemoryAttributeStore returns an in-memory AttributeStore.
func NewMemoryAttributeStore() AttributeStore {
	return &memoryAttributeStore{
		data: make(map[string]map[model.AttributeScope]map[string]model.AttributeKv),
	}
}

func (s *memoryAttributeStore) SaveAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, attrs map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	eKey := entityID.String()
	if _, ok := s.data[eKey]; !ok {
		s.data[eKey] = make(map[model.AttributeScope]map[string]model.AttributeKv)
	}
	if _, ok := s.data[eKey][scope]; !ok {
		s.data[eKey][scope] = make(map[string]model.AttributeKv)
	}

	now := time.Now().UnixMilli()
	for k, v := range attrs {
		s.data[eKey][scope][k] = model.AttributeKv{
			EntityID:   entityID,
			Scope:      scope,
			Key:        k,
			LastUpdate: now,
			Val:        v,
		}
	}
	return nil
}

func (s *memoryAttributeStore) GetAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, keys []string) ([]model.AttributeKv, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	eKey := entityID.String()
	scopeMap, ok := s.data[eKey]
	if !ok {
		return []model.AttributeKv{}, nil
	}

	attrsMap, ok := scopeMap[scope]
	if !ok {
		return []model.AttributeKv{}, nil
	}

	var results []model.AttributeKv
	if len(keys) == 0 {
		for _, kv := range attrsMap {
			results = append(results, kv)
		}
	} else {
		for _, k := range keys {
			if kv, exists := attrsMap[k]; exists {
				results = append(results, kv)
			}
		}
	}
	return results, nil
}

func (s *memoryAttributeStore) GetAllAttributes(ctx context.Context, entityID ids.ID) ([]model.AttributeKv, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	eKey := entityID.String()
	scopeMap, ok := s.data[eKey]
	if !ok {
		return []model.AttributeKv{}, nil
	}

	var results []model.AttributeKv
	for _, m := range scopeMap {
		for _, kv := range m {
			results = append(results, kv)
		}
	}
	return results, nil
}

func (s *memoryAttributeStore) DeleteAttributes(ctx context.Context, entityID ids.ID, scope model.AttributeScope, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	eKey := entityID.String()
	scopeMap, ok := s.data[eKey]
	if !ok {
		return nil
	}

	attrsMap, ok := scopeMap[scope]
	if !ok {
		return nil
	}

	for _, k := range keys {
		delete(attrsMap, k)
	}
	return nil
}
