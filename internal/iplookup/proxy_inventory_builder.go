package iplookup

import (
	"context"
	"net/netip"
	"sort"
	"strings"
)

type inventoryBuilder struct {
	index   *prefixIndex
	memory  *prefixIndexBuilder
	disk    *diskWriter
	domains map[string]bool
	err     error
}

func (s *EnhancedService) sourceEntryLimit(c Config) int {
	if s.diskRequired {
		return maxInventoryEntries
	}
	return proxySourceEntryLimit(len(c.ProxySources))
}

func (s *EnhancedService) newInventoryBuilder(ctx context.Context, limit int) *inventoryBuilder {
	b := &inventoryBuilder{}
	if s.diskRequired {
		if s.diskError != nil || s.disk == nil {
			b.err = &CodeError{"IP_LOOKUP_UNAVAILABLE"}
			return b
		}
		index, err := s.disk.newGeneration(ctx)
		if err != nil {
			b.err = err
			return b
		}
		b.index = index
		b.disk = &diskWriter{ctx: ctx, index: index, limit: limit}
	} else {
		b.memory = newPrefixIndexBuilder(limit)
		b.index = &b.memory.index
		b.domains = map[string]bool{}
	}
	return b
}

func (b *inventoryBuilder) add(prefix netip.Prefix) bool {
	if b.err != nil {
		return false
	}
	if b.disk != nil {
		return b.disk.add(prefix)
	}
	return b.memory.add(prefix)
}

func (b *inventoryBuilder) addDomain(host string) bool {
	if b.err != nil {
		return false
	}
	// Detach canonical hosts from large parser-owned backing strings.
	host = strings.Clone(host)
	if b.disk != nil {
		b.disk.addDomain(host)
		return b.disk.err == nil
	}
	if len(b.domains) >= maxProxyHosts && !b.domains[host] {
		return false
	}
	b.domains[host] = true
	return true
}

func (b *inventoryBuilder) domainCount() int {
	if b.disk != nil {
		return b.disk.hosts
	}
	return len(b.domains)
}

func (b *inventoryBuilder) flush() {
	if b.disk != nil {
		b.disk.flush()
		if b.disk.err != nil {
			b.err = b.disk.err
		}
	}
}

func (b *inventoryBuilder) eachDomain(ctx context.Context, visit func(string) bool) error {
	if b.disk != nil {
		after := ""
		for {
			hosts, err := b.disk.domainPage(after)
			if err != nil {
				return err
			}
			if len(hosts) == 0 {
				return nil
			}
			for _, host := range hosts {
				if !visit(host) {
					return ctx.Err()
				}
			}
			after = hosts[len(hosts)-1]
		}
	}
	hosts := make([]string, 0, len(b.domains))
	for host := range b.domains {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	for _, host := range hosts {
		if !visit(host) {
			return ctx.Err()
		}
	}
	return nil
}

func (b *inventoryBuilder) freeze() *prefixIndex {
	b.flush()
	if b.memory != nil {
		return b.memory.freeze()
	}
	if b.disk != nil {
		if err := b.disk.clearDomains(); err != nil && b.err == nil {
			b.err = err
		}
	}
	return b.index
}
