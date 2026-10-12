package rule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

type memoryStore struct {
	mu        sync.RWMutex
	chains    map[ids.ID]*model.RuleChain
	revisions map[ids.ID]map[int]*model.RuleChainRevision
}

// NewMemoryStore creates an in-memory Store.
func NewMemoryStore() Store {
	return &memoryStore{
		chains:    make(map[ids.ID]*model.RuleChain),
		revisions: make(map[ids.ID]map[int]*model.RuleChainRevision),
	}
}

func (s *memoryStore) ListRuleChains(ctx context.Context, tenantID ids.ID, page, limit int) ([]*model.RuleChain, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tenantChains []*model.RuleChain
	for _, c := range s.chains {
		if c.TenantID == tenantID {
			tenantChains = append(tenantChains, c)
		}
	}

	total := len(tenantChains)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	start := (page - 1) * limit
	if start >= total {
		return []*model.RuleChain{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return tenantChains[start:end], total, nil
}

func (s *memoryStore) GetRuleChain(ctx context.Context, tenantID, id ids.ID) (*model.RuleChain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.chains[id]
	if !ok || c.TenantID != tenantID {
		return nil, fmt.Errorf("rule chain %s not found", id)
	}
	return c, nil
}

func (s *memoryStore) GetRootRuleChain(ctx context.Context, tenantID ids.ID) (*model.RuleChain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.chains {
		if c.TenantID == tenantID && c.IsRoot {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no root rule chain found for tenant %s", tenantID)
}

func (s *memoryStore) SaveRuleChain(ctx context.Context, chain *model.RuleChain) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if chain.ID == ids.Nil {
		chain.ID = ids.MustNew()
	}
	if chain.CreatedTime == 0 {
		chain.CreatedTime = time.Now().UnixMilli()
	}
	if chain.PublishedRevision == 0 {
		chain.PublishedRevision = 1
	}

	// If marked root, unset others for tenant
	if chain.IsRoot {
		for _, c := range s.chains {
			if c.TenantID == chain.TenantID && c.ID != chain.ID {
				c.IsRoot = false
			}
		}
	}

	s.chains[chain.ID] = chain
	return nil
}

func (s *memoryStore) SetRootRuleChain(ctx context.Context, tenantID, id ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for _, c := range s.chains {
		if c.TenantID == tenantID {
			if c.ID == id {
				c.IsRoot = true
				found = true
			} else {
				c.IsRoot = false
			}
		}
	}
	if !found {
		return fmt.Errorf("rule chain %s not found", id)
	}
	return nil
}

func (s *memoryStore) DeleteRuleChain(ctx context.Context, tenantID, id ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.chains[id]
	if !ok || c.TenantID != tenantID {
		return fmt.Errorf("rule chain %s not found", id)
	}
	delete(s.chains, id)
	delete(s.revisions, id)
	return nil
}

func (s *memoryStore) GetRevision(ctx context.Context, chainID ids.ID, revision int) (*model.RuleChainRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chainRevs, ok := s.revisions[chainID]
	if !ok {
		return nil, fmt.Errorf("revisions for rule chain %s not found", chainID)
	}
	rev, ok := chainRevs[revision]
	if !ok {
		return nil, fmt.Errorf("revision %d for rule chain %s not found", revision, chainID)
	}
	return rev, nil
}

func (s *memoryStore) GetLatestRevision(ctx context.Context, chainID ids.ID) (*model.RuleChainRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chainRevs, ok := s.revisions[chainID]
	if !ok || len(chainRevs) == 0 {
		return nil, fmt.Errorf("no revisions found for rule chain %s", chainID)
	}

	maxRev := 0
	var latest *model.RuleChainRevision
	for revNum, rev := range chainRevs {
		if revNum > maxRev {
			maxRev = revNum
			latest = rev
		}
	}
	return latest, nil
}

func (s *memoryStore) SaveRevision(ctx context.Context, rev *model.RuleChainRevision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rev.CreatedTime == 0 {
		rev.CreatedTime = time.Now().UnixMilli()
	}

	if s.revisions[rev.RuleChainID] == nil {
		s.revisions[rev.RuleChainID] = make(map[int]*model.RuleChainRevision)
	}
	s.revisions[rev.RuleChainID][rev.Revision] = rev

	// Update chain's published revision if chain exists
	if c, ok := s.chains[rev.RuleChainID]; ok {
		if rev.Revision > c.PublishedRevision {
			c.PublishedRevision = rev.Revision
		}
	}

	return nil
}
