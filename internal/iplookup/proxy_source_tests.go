package iplookup

import (
	"context"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
)

type sourceTestEntry struct {
	test    ProxySourceTest
	expires time.Time
}
type sourceTestRuntime struct {
	ctx     context.Context
	mu      sync.Mutex
	work    sync.WaitGroup
	active  bool
	entries map[string]sourceTestEntry
}

// StartSourceTests attaches diagnostics to the application lifecycle.
func (s *EnhancedService) StartSourceTests(ctx context.Context) {
	s.tests = &sourceTestRuntime{ctx: ctx, entries: map[string]sourceTestEntry{}}
}

// WaitSourceTests joins diagnostics before application dependencies close.
func (s *EnhancedService) WaitSourceTests() {
	if s.tests != nil {
		s.tests.work.Wait()
	}
}

// TestSource starts a fresh full diagnostic using saved settings without billing.
func (s *EnhancedService) TestSource(ctx context.Context, id string) (ProxySourceTest, error) {
	b := s.tests
	if b == nil || b.ctx.Err() != nil {
		return ProxySourceTest{}, &CodeError{"IP_LOOKUP_UNAVAILABLE"}
	}
	raw, err := s.settings.Optional(ctx, SettingKey)
	if err != nil {
		return ProxySourceTest{}, err
	}
	c, err := DecodeConfig(raw)
	if err != nil {
		return ProxySourceTest{}, err
	}
	var source *ProxySource
	for _, candidate := range c.ProxySources {
		if candidate.ID == id {
			value := candidate
			source = &value
			break
		}
	}
	if source == nil {
		return ProxySourceTest{}, &CodeError{"IP_PROXY_SOURCE_NOT_FOUND"}
	}
	testID, err := ids.New()
	if err != nil {
		return ProxySourceTest{}, err
	}
	test := ProxySourceTest{ID: testID, Status: "queued", Result: ProxySourceResult{ID: source.ID, Name: source.Name, URL: safeSourceURL(source.URL), Status: "pending", Phase: "download"}}
	b.mu.Lock()
	if b.active || b.ctx.Err() != nil {
		b.mu.Unlock()
		return ProxySourceTest{}, &CodeError{"IP_LOOKUP_BUSY"}
	}
	for id, entry := range b.entries {
		if !entry.expires.IsZero() && !entry.expires.After(time.Now()) {
			delete(b.entries, id)
		}
	}
	if len(b.entries) >= 64 {
		b.mu.Unlock()
		return ProxySourceTest{}, &CodeError{"IP_LOOKUP_BUSY"}
	}
	b.active = true
	b.entries[testID] = sourceTestEntry{test: test}
	b.work.Add(1)
	b.mu.Unlock()
	go s.runSourceTest(b, test, *source, c)
	return test, nil
}

// SourceTest reads an unexpired administrator diagnostic.
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
