package xnode

type routingAssetDomainKey struct{ source, hash, selector string }

type routingAssetDomains struct {
	values      []string
	bytes, refs int
}

func (a *routingAssets) remove(key string) {
	entry := a.entries[key]
	if entry == nil {
		return
	}
	for selector := range entry.data.domains {
		id := routingAssetDomainKey{entry.source, entry.data.hash, selector}
		shared := a.domains[id]
		shared.refs--
		if shared.refs == 0 {
			a.bytes -= shared.bytes
			delete(a.domains, id)
		}
	}
	a.bytes -= entry.bytes
	delete(a.entries, key)
}

func (a *routingAssets) retain(key string, entry *routingAssetEntry) {
	a.remove(key)
	if a.entries == nil {
		a.entries = map[string]*routingAssetEntry{}
		a.domains = map[routingAssetDomainKey]*routingAssetDomains{}
	}
	entry.bytes = 256 + len(key) + len(entry.source)
	if entry.data.domains != nil {
		canonical := make(map[string][]string, len(entry.data.domains))
		for selector, domains := range entry.data.domains {
			id := routingAssetDomainKey{entry.source, entry.data.hash, selector}
			shared := a.domains[id]
			if shared == nil {
				// Account for backing capacity and string/map headers as well as
				// text. One verified source/hash/selector owns one immutable slice.
				size := 128 + len(id.source) + len(id.hash) + len(selector) + 16*cap(domains)
				for _, domain := range domains {
					size += len(domain)
				}
				shared = &routingAssetDomains{values: domains, bytes: size}
				a.domains[id] = shared
				a.bytes += size
			}
			shared.refs++
			canonical[selector] = shared.values
			entry.bytes += 64 + len(selector)
		}
		entry.data.domains = canonical
	}
	a.entries[key] = entry
	a.bytes += entry.bytes
	a.evict()
	if a.entries[key] == nil {
		// A valid projection can exceed the retention budget. Its caller still
		// gets the full verified result; retain only a refresh fence/generation.
		// The nil domain map forces a new load, never an incomplete cache hit.
		metadata := *entry
		metadata.data = routingAssetResult{hash: entry.data.hash}
		metadata.bytes = 256 + len(key) + len(entry.source)
		a.entries[key] = &metadata
		a.bytes += metadata.bytes
		a.evict()
	}
	// Source generation metadata is bounded by the resident key inventory,
	// including metadata-only entries for projections above the byte budget.
	for source := range a.sources {
		present := false
		for _, cached := range a.entries {
			present = present || cached.source == source
		}
		if !present {
			delete(a.sources, source)
		}
	}
}

func (a *routingAssets) evict() {
	for len(a.entries) > maxRoutingAssetEntries || a.bytes > maxRoutingAssetCacheBytes {
		oldest := ""
		var used uint64
		for candidate, cached := range a.entries {
			if oldest == "" || cached.used < used {
				oldest, used = candidate, cached.used
			}
		}
		a.remove(oldest)
	}
}
