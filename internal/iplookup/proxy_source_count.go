package iplookup

// refreshCount counts unique prefixes only when published evidence changes.
// No expensive work holds the runtime mutex or runs in an admin polling read.
func (b *sourceTestRuntime) refreshCount() {
	b.countMu.Lock()
	defer b.countMu.Unlock()
	for {
		b.mu.Lock()
		revision := b.revision
		if revision == b.counted {
			b.mu.Unlock()
			return
		}
		indexes := make([]*prefixIndex, 0, len(b.sources))
		for _, cache := range b.sources {
			if cache.source.Enabled && cache.index != nil {
				indexes = append(indexes, cache.index)
			}
		}
		b.mu.Unlock()
		count := countPrefixIndexes(indexes)
		b.mu.Lock()
		if b.revision == revision {
			b.uniqueEntries, b.counted = count, revision
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()
	}
}
