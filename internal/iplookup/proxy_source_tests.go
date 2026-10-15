package iplookup

import (
	"context"
	"sync"
	"time"
)

type sourceTestEntry struct {
	test    ProxySourceTest
	expires time.Time
}
type sourceCache struct {
	source               ProxySource
	key                  [32]byte
	index                *prefixIndex
	result               ProxySourceResult
	complete             bool
	expires, nextRefresh time.Time
	jobID                string
	done                 chan struct{}
	cancel               context.CancelFunc
	automatic            bool
}

func (c *sourceCache) retireIndex() {
	c.index.release()
	c.index, c.complete = nil, false
}

type sourceRefreshJob struct {
	cache      *sourceCache
	source     ProxySource
	id         string
	done       chan struct{}
	cancel     context.CancelFunc
	entryLimit int
}
type sourceTestRuntime struct {
	ctx               context.Context
	mu, configMu      sync.Mutex
	countMu           sync.Mutex
	revision, counted uint64
	uniqueEntries     int
	countError        error
	work              sync.WaitGroup
	entries           map[string]sourceTestEntry
	sources           map[string]*sourceCache
	slots             chan struct{}
}

// StartSourceTests starts shared refreshes independently of member traffic.
func (s *EnhancedService) StartSourceTests(ctx context.Context) {
	b := &sourceTestRuntime{ctx: ctx, entries: map[string]sourceTestEntry{}, sources: map[string]*sourceCache{}, slots: make(chan struct{}, 1)}
	s.tests = b
	b.work.Add(1)
	go func() {
		defer b.work.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			s.pruneInventoryMemos()
			_ = s.ReconcileSources(ctx) // Saved configuration is retried on the next tick.
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// WaitSourceTests joins the scheduler and refreshes before queues/settings close.
func (s *EnhancedService) WaitSourceTests() {
	if s.tests != nil {
		s.tests.work.Wait()
	}
}

// ReconcileSources schedules due sources and invalidates changed configurations.
func (s *EnhancedService) ReconcileSources(ctx context.Context) error {
	b := s.tests
	if b == nil {
		return &CodeError{"IP_LOOKUP_UNAVAILABLE"}
	}
	b.configMu.Lock()
	defer b.configMu.Unlock()
	c, err := s.sourceConfig(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.ctx.Err() != nil {
		b.mu.Unlock()
		return b.ctx.Err()
	}
	s.reconcileSourcesLocked(b, c)
	b.mu.Unlock()
	b.refreshCount()
	return nil
}

func (s *EnhancedService) sourceConfig(ctx context.Context) (Config, error) {
	if s.settings == nil {
		return Config{}, &CodeError{"IP_LOOKUP_UNAVAILABLE"}
	}
	raw, err := s.settings.Optional(ctx, SettingKey)
	if err != nil {
		return Config{}, err
	}
	return DecodeConfig(raw)
}

// TestSource refreshes one saved source, reusing an in-flight refresh.
func (s *EnhancedService) TestSource(ctx context.Context, id string) (ProxySourceTest, error) {
	b := s.tests
	if b == nil {
		return ProxySourceTest{}, &CodeError{"IP_LOOKUP_UNAVAILABLE"}
	}
	b.configMu.Lock()
	defer b.configMu.Unlock()
	c, err := s.sourceConfig(ctx)
	if err != nil {
		return ProxySourceTest{}, err
	}
	b.mu.Lock()
	if b.ctx.Err() != nil {
		b.mu.Unlock()
		return ProxySourceTest{}, &CodeError{"IP_LOOKUP_UNAVAILABLE"}
	}
	s.reconcileSourcesLocked(b, c)
	cache := b.sources[id]
	if cache == nil {
		b.mu.Unlock()
		b.refreshCount()
		return ProxySourceTest{}, &CodeError{"IP_PROXY_SOURCE_NOT_FOUND"}
	}
	test, err := s.startSourceLocked(b, cache, c, false)
	b.mu.Unlock()
	b.refreshCount()
	return test, err
}

// SourceTest reads a refresh operation without triggering collection.
func (s *EnhancedService) SourceTest(id string) (ProxySourceTest, error) {
	if s.tests == nil {
		return ProxySourceTest{}, &CodeError{"IP_PROXY_SOURCE_NOT_FOUND"}
	}
	b := s.tests
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.entries[id]
	if !ok || (!entry.expires.IsZero() && !entry.expires.After(time.Now())) {
		delete(b.entries, id)
		return ProxySourceTest{}, &CodeError{"IP_PROXY_SOURCE_NOT_FOUND"}
	}
	return entry.test, nil
}
