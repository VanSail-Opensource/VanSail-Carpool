package iplookup

import "context"

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
				indexes = append(indexes, cache.index.retain())
			}
		}
		b.mu.Unlock()
		ctx := b.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		count, err := countPrefixIndexesContext(ctx, indexes)
		for _, index := range indexes {
			index.release()
		}
		b.mu.Lock()
		if b.revision == revision {
			b.uniqueEntries, b.counted, b.countError = count, revision, err
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()
	}
}
