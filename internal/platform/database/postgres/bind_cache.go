package postgres

import (
	"container/list"
	"database/sql/driver"
	"sync"
)

const bindCacheEntries = 256
const bindCacheBytes = 2 << 20

type bindKey struct{ query, kinds string }
type boundSQL struct {
	query string
	bools []int
}

// bindingCache belongs to one pool. Only SQL grammar and argument types are
// retained; argument values, balances and query results never enter this cache.
type bindingCache struct {
	mu    sync.Mutex
	items map[bindKey]boundSQL
	order list.List
	bytes int
}

func (c *bindingCache) bind(query string, args []driver.NamedValue) (string, error) {
	if len(query) > 16<<10 || len(args) > 256 {
		return Bind(query, args)
	}
	kinds := make([]byte, len(args))
	for i, arg := range args {
		switch arg.Value.(type) {
		case bool:
			kinds[i] = 'b'
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			kinds[i] = 'i'
		default:
			kinds[i] = 'v'
		}
	}
	key := bindKey{query, string(kinds)}
	c.mu.Lock()
	saved, found := c.items[key]
	c.mu.Unlock()
	if found {
		for _, i := range saved.bools {
			flag := int64(0)
			if args[i].Value.(bool) {
				flag = 1
			}
			args[i].Value = flag
		}
		return saved.query, nil
	}
	bound, err := Bind(query, args)
	if err != nil {
		return "", err
	}
	entry := boundSQL{query: bound}
	for i, kind := range kinds {
		if kind == 'b' {
			if _, converted := args[i].Value.(int64); converted {
				entry.bools = append(entry.bools, i)
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = make(map[bindKey]boundSQL)
	}
	if _, exists := c.items[key]; exists {
		return bound, nil
	}
	c.items[key] = entry
	c.order.PushBack(key)
	c.bytes += len(query) + len(key.kinds) + len(bound)
	for len(c.items) > bindCacheEntries || c.bytes > bindCacheBytes {
		oldest := c.order.Front()
		oldKey := oldest.Value.(bindKey)
		old := c.items[oldKey]
		c.bytes -= len(oldKey.query) + len(oldKey.kinds) + len(old.query)
		delete(c.items, oldKey)
		c.order.Remove(oldest)
	}
	return bound, nil
}
