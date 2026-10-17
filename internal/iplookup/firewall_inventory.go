package iplookup

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"net/netip"
	"time"
)

// FirewallSources borrows immutable published inventories. No credentials or
// source documents cross this interface; CIDRs retain their stored granularity.
func (s *EnhancedService) FirewallSources(ctx context.Context, visit func(wire.FirewallSource, func(func([]string) error) error) error) ([]string, error) {
	config, err := s.sourceConfig(ctx)
	if err != nil {
		return nil, err
	}
	var allowed []string
	for _, source := range config.ProxySources {
		if source.Enabled {
			allowed = append(allowed, source.ID)
		}
	}
	b := s.tests
	if b == nil {
		return allowed, nil
	}
	type borrowed struct {
		source wire.FirewallSource
		index  *prefixIndex
	}
	var snapshots []borrowed
	b.mu.Lock()
	for _, source := range config.ProxySources {
		cache := b.sources[source.ID]
		if !source.Enabled || cache == nil || !cache.complete || cache.index == nil || !cache.expires.After(time.Now()) || cache.key != s.sourceKey(source, config) {
			continue
		}
		at := cache.expires.Add(-proxyInventoryTTL).UTC()
		hash := sha256.Sum256([]byte(hex.EncodeToString(cache.key[:]) + at.Format(time.RFC3339Nano)))
		snapshots = append(snapshots, borrowed{wire.FirewallSource{ID: source.ID, Generation: "source-" + hex.EncodeToString(hash[:16]), Until: at.Add(72 * time.Hour), RefreshSeconds: 72 * 3600}, cache.index.retain()})
	}
	b.mu.Unlock()
	defer func() {
		for _, item := range snapshots {
			item.index.release()
		}
	}()
	for _, item := range snapshots {
		if err = visit(item.source, func(send func([]string) error) error { return item.index.walkPrefixes(ctx, send) }); err != nil {
			return allowed, err
		}
	}
	return allowed, nil
}
func (p *prefixIndex) walkPrefixes(ctx context.Context, send func([]string) error) error {
	if p.disk != nil {
		return p.walkDiskPrefixes(ctx, send)
	}
	page := make([]string, 0, 2048)
	emit := func(prefix netip.Prefix) error {
		page = append(page, prefix.String())
		if len(page) < cap(page) {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		err := send(page)
		page = make([]string, 0, 2048)
		return err
	}
	for bits, bucket := range p.v4 {
		if bucket == nil {
			continue
		}
		it := bucket.Iterator()
		for it.HasNext() {
			var raw [4]byte
			binary.BigEndian.PutUint32(raw[:], it.Next()<<uint(32-bits))
			if err := emit(netip.PrefixFrom(netip.AddrFrom4(raw), bits)); err != nil {
				return err
			}
		}
	}
	for bits, bucket := range p.v6 {
		for _, raw := range bucket {
			if err := emit(netip.PrefixFrom(netip.AddrFrom16(raw), bits)); err != nil {
				return err
			}
		}
	}
	if len(page) > 0 || p.count() == 0 {
		return send(page)
	}
	return nil
}
