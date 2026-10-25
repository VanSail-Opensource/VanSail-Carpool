package xnode

import (
	"context"
	"encoding/json"
	"sync"
)

const topologyCacheBytes = 4 << 20

// CatalogTopology excludes credentials, agent status and member decisions.
type CatalogTopology struct {
	Nodes  []TopologyNode
	Groups []Group
}

type TopologyNode struct {
	ID, Name, CountryCode, ProviderName string
	Multiplier                          float64
	Enabled                             bool
}

// One immutable encoded snapshot bounds retention and gives callers isolated
// copies. Loading is deduplicated; canceled waiters do not wait for the loader.
type topologyCache struct {
	mu      sync.Mutex
	version int64
	raw     []byte
	flight  chan struct{}
}

func (s *Service) CatalogTopology(ctx context.Context) (CatalogTopology, error) {
	repo, supported := s.repo.(compileVersionRepository)
	if !supported {
		return s.readCatalogTopology(ctx)
	}
	c := s.topology
	for attempt := 0; attempt < 3; {
		version, _, err := repo.XNodeCompileVersion(ctx, s.now())
		if err != nil {
			return CatalogTopology{}, err
		}
		c.mu.Lock()
		if c.raw != nil && c.version == version {
			raw := c.raw
			c.mu.Unlock()
			var result CatalogTopology
			err = json.Unmarshal(raw, &result)
			return result, err
		}
		if pending := c.flight; pending != nil {
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return CatalogTopology{}, ctx.Err()
			case <-pending:
				continue
			}
		}
		c.flight = make(chan struct{})
		c.mu.Unlock()
		result, err := s.readCatalogTopology(ctx)
		var raw []byte
		if err == nil {
			raw, err = json.Marshal(result)
		}
		stable := false
		if err == nil {
			var after int64
			after, _, err = repo.XNodeCompileVersion(ctx, s.now())
			stable = after == version
		}
		c.mu.Lock()
		if err == nil && stable && len(raw) <= topologyCacheBytes {
			c.version, c.raw = version, raw
		} else {
			c.raw = nil
		}
		close(c.flight)
		c.flight = nil
		c.mu.Unlock()
		if err != nil {
			return CatalogTopology{}, err
		}
		if stable {
			return result, nil
		}
		attempt++
	}
	return CatalogTopology{}, ErrConflict
}

func (s *Service) readCatalogTopology(ctx context.Context) (CatalogTopology, error) {
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return CatalogTopology{}, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return CatalogTopology{}, err
	}
	result := CatalogTopology{Nodes: make([]TopologyNode, 0, len(nodes)), Groups: groups}
	for _, node := range nodes {
		result.Nodes = append(result.Nodes, TopologyNode{ID: node.ID, Name: node.Name,
			CountryCode: node.CountryCode, ProviderName: node.ProviderName, Multiplier: node.Multiplier, Enabled: node.Enabled})
	}
	return result, nil
}
