package pricing

import (
	"container/list"
	"context"
	"strconv"
	"sync"
	"time"
)

type analysisEntry struct {
	value   []byte
	until   time.Time
	element *list.Element
}
type analysisFlight struct {
	done  chan struct{}
	value []byte
	err   error
}

// AnalysisCache bounds read-only analytical results to 8 MiB and 128 keys.
// Concurrent readers share calculation. Billing, policy, quotes, health and
// discount budgets never use this cache as an authorization source.
type AnalysisCache struct {
	mu                   sync.Mutex
	entries              map[string]analysisEntry
	flights              map[string]*analysisFlight
	bytes                int
	generation           uint64
	order                list.List
	maxEntries, maxBytes int
}

func NewAnalysisCache() *AnalysisCache {
	return &AnalysisCache{entries: map[string]analysisEntry{}, flights: map[string]*analysisFlight{}, maxEntries: 128, maxBytes: 8 << 20}
}
func (c *AnalysisCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order.Init()
	c.entries = map[string]analysisEntry{}
	c.bytes = 0
	c.generation++
}
func (c *AnalysisCache) Load(ctx context.Context, key string, ttl time.Duration, load func() ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	if e, ok := c.entries[key]; ok && time.Now().Before(e.until) {
		c.mu.Unlock()
		return append([]byte(nil), e.value...), nil
	}
	flightKey := strconv.FormatUint(c.generation, 10) + ":" + key
	if f, ok := c.flights[flightKey]; ok {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.done:
			return append([]byte(nil), f.value...), f.err
		}
	}
	if len(c.flights) >= 64 {
		c.mu.Unlock()
		return nil, WorkerResourceLimit
	}
	f := &analysisFlight{done: make(chan struct{})}
	c.flights[flightKey] = f
	generation := c.generation
	c.mu.Unlock()
	value, err := load()
	c.mu.Lock()
	defer c.mu.Unlock()
	f.value, f.err = append([]byte(nil), value...), err
	delete(c.flights, flightKey)
	defer close(f.done)
	if err == nil && generation == c.generation && len(value) <= 1<<20 {
		if old, ok := c.entries[key]; ok {
			c.bytes -= len(old.value) + len(key) + 128
			c.order.Remove(old.element)
			delete(c.entries, key)
		}
		for len(c.entries) > 0 && (len(c.entries) >= c.maxEntries || c.bytes+len(value)+len(key)+128 > c.maxBytes) {
			oldest := c.order.Front()
			oldKey := oldest.Value.(string)
			c.bytes -= len(c.entries[oldKey].value) + len(oldKey) + 128
			delete(c.entries, oldKey)
			c.order.Remove(oldest)
		}
		c.entries[key] = analysisEntry{value: append([]byte(nil), value...), until: time.Now().Add(ttl), element: c.order.PushBack(key)}
		c.bytes += len(value) + len(key) + 128
	}
	return value, err
}

// Exposure cache stores only bounded deduplication markers, never prices or credentials.
func NewExposureCache() *AnalysisCache {
	c := NewAnalysisCache()
	c.maxEntries = 16384
	c.maxBytes = 4 << 20
	return c
}
