package iplookup

// refreshCount performs compressed unions only when published evidence changes.
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
		var combined *prefixIndex
		for _, index := range indexes {
			combined = mergePrefixIndexes(combined, index, 0)
		}
		b.mu.Lock()
		if b.revision == revision {
			b.uniqueEntries, b.counted = combined.count(), revision
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()
	}
}
