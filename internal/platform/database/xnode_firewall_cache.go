package database

import (
	"container/list"
	"context"
	"slices"
	"sync"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const firewallPageCacheBytes = 8 << 20
const firewallPageCacheEntries = 128
const firewallPageCacheAge = 5 * time.Minute

type firewallPageKey struct {
	after, target uint64
	full          bool
	cursor        string
	limit         int
}
type firewallPageEntry struct {
	key   firewallPageKey
	page  wire.FirewallUpdate
	next  string
	at    time.Time
	bytes int
}
type firewallPageCache struct {
	mu         sync.Mutex
	load       sync.Mutex
	generation uint64
	bytes      int
	entries    map[firewallPageKey]*list.Element
	lru        list.List
}

func (c *firewallPageCache) get(version uint64, key firewallPageKey, now time.Time) (wire.FirewallUpdate, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Version is read from the committed clock, never from an uncommitted write.
	// A delayed older reader cannot invalidate a newer published generation.
	if version > c.generation {
		c.generation, c.bytes, c.entries = version, 0, nil
		c.lru.Init()
	}
	entry := c.entries[key]
	if entry == nil || version != c.generation {
		return wire.FirewallUpdate{}, "", false
	}
	value := entry.Value.(firewallPageEntry)
	if now.Before(value.at) || now.Sub(value.at) >= firewallPageCacheAge {
		c.remove(entry)
		return wire.FirewallUpdate{}, "", false
	}
	c.lru.MoveToFront(entry)
	return cloneFirewallPage(value.page), value.next, true
}

func (c *firewallPageCache) read(ctx context.Context, version uint64, key firewallPageKey, load func() (wire.FirewallUpdate, string, error)) (wire.FirewallUpdate, string, error) {
	if page, next, ok := c.get(version, key, time.Now()); ok {
		return page, next, nil
	}
	c.load.Lock()
	defer c.load.Unlock()
	if err := ctx.Err(); err != nil {
		return wire.FirewallUpdate{}, "", err
	}
	if page, next, ok := c.get(version, key, time.Now()); ok {
		return page, next, nil
	}
	page, next, err := load()
	if err != nil {
		return page, next, err
	}
	c.put(version, key, page, next)
	return page, next, nil
}

func (c *firewallPageCache) put(version uint64, key firewallPageKey, page wire.FirewallUpdate, next string) {
	size := len(key.cursor) + len(next) + 256
	for _, ban := range page.Bans {
		size += len(ban.Prefix) + len(ban.Generation) + 128
	}
	for _, grant := range page.Exemptions {
		size += len(grant.IP) + len(grant.Generation) + 128
	}
	for _, items := range [][]string{page.Remove, page.RemoveExemptions} {
		for _, item := range items {
			size += len(item) + 32
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if version != c.generation || size > firewallPageCacheBytes {
		return
	}
	for c.lru.Len() >= firewallPageCacheEntries || c.bytes+size > firewallPageCacheBytes {
		c.remove(c.lru.Back())
	}
	if c.entries == nil {
		c.entries = map[firewallPageKey]*list.Element{}
	}
	c.entries[key] = c.lru.PushFront(firewallPageEntry{key, cloneFirewallPage(page), next, time.Now(), size})
	c.bytes += size
}

func (c *firewallPageCache) remove(entry *list.Element) {
	value := entry.Value.(firewallPageEntry)
	c.bytes -= value.bytes
	delete(c.entries, value.key)
	c.lru.Remove(entry)
}

func cloneFirewallPage(page wire.FirewallUpdate) wire.FirewallUpdate {
	// Database pages contain only these four banks. Source inventories are retired.
	page.Bans = slices.Clone(page.Bans)
	page.Exemptions = slices.Clone(page.Exemptions)
	page.Remove = slices.Clone(page.Remove)
	page.RemoveExemptions = slices.Clone(page.RemoveExemptions)
	return page
}
