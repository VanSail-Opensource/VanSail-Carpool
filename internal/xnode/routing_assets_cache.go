package xnode

import (
	"context"
	"sync"
	"time"
)

const maxRoutingAssetEntries = 32
const maxRoutingAssetCacheBytes = 16 << 20

// Only verified selector expansions are retained, never whole downloaded assets
// or member authorization. Different members must not evict a single global key.
type routingAssets struct {
	mu      sync.Mutex
	entries map[string]*routingAssetEntry
	sources map[string]string
	domains map[routingAssetDomainKey]*routingAssetDomains
	clock   uint64
	bytes   int
	publish func()
}

type routingAssetEntry struct {
	source   string
	data     routingAssetResult
	next     time.Time
	failures int
	err      error
	used     uint64
	bytes    int
}

type routingAssetLoader func(context.Context, GlobalConfig, []string) (routingAssetResult, error)

func (a *routingAssets) resolve(ctx context.Context, config GlobalConfig, selectors []string, now func() time.Time, load routingAssetLoader) (routingAssetResult, error) {
	if err := ctx.Err(); err != nil {
		return routingAssetResult{}, err
	}
	source, key := routingAssetKeys(config, selectors)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return routingAssetResult{}, err
	}
	a.clock++
	entry := a.entries[key]
	if entry != nil && (entry.err != nil || entry.data.domains != nil) && (entry.next.IsZero() || now().Before(entry.next)) {
		entry.used = a.clock
		return entry.data, entry.err
	}
	if entry == nil {
		entry = &routingAssetEntry{source: source}
	} else {
		copy := *entry
		entry = &copy
	}
	// Keep the existing serialized queued loader. A waiting reader rechecks the
	// entry after this refresh, so concurrent calls cannot repeat the download.
	data, err := load(ctx, config, selectors)
	if ctx.Err() != nil {
		return routingAssetResult{}, ctx.Err()
	}
	completed := now()
	next := nextMasterGeodata(config, completed)
	if err != nil {
		if entry.data.domains == nil {
			entry.data = routingAssetResult{}
			entry.err = err
			next = completed.Add(routingAssetRetryDelay(entry.failures))
			entry.failures = min(entry.failures+1, 3)
		} else if !next.IsZero() && entry.failures < 3 {
			next = completed.Add(routingAssetRetryDelay(entry.failures))
			entry.failures++
		} else {
			entry.failures = 0
		}
	} else {
		previous := a.sources[source]
		if previous != "" && previous != data.hash {
			// One source generation cannot mix old selector expansions with a
			// newly verified file. Unrelated source configurations stay isolated.
			for oldKey, old := range a.entries {
				if old.source == source {
					a.remove(oldKey)
				}
			}
		}
		entry.data, entry.err, entry.failures = data, nil, 0
		if a.sources == nil {
			a.sources = map[string]string{}
		}
		a.sources[source] = data.hash
		if previous != data.hash {
			if a.publish != nil {
				a.publish()
			}
		}
	}
	entry.next, entry.used = next, a.clock
	a.retain(key, entry)
	return entry.data, entry.err
}

func routingAssetRetryDelay(failures int) time.Duration {
	return []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}[min(failures, 2)]
}
