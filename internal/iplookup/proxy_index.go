package iplookup

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/netip"
	"slices"

	"github.com/RoaringBitmap/roaring/v2"
)

const maxInventoryEntries = 5000000
const maxAggregateInventoryEntries = 10000000

func proxySourceEntryLimit(sourceCount int) int {
	if sourceCount < 1 {
		return maxInventoryEntries
	}
	return min(maxInventoryEntries, max(1, maxAggregateInventoryEntries/sourceCount))
}

// prefixIndex is immutable after publication. Prefix lengths remain separate:
// overlapping CIDRs retain both their attribution and original entry counts.
type prefixIndex struct {
	diskBits [2][129]bool
	disk     *diskGeneration
	v4       [33]*roaring.Bitmap
	v6       [129][][16]byte
	size     int
}

func (p *prefixIndex) count() int {
	if p == nil {
		return 0
	}
	return p.size
}

func (p *prefixIndex) contains(addr netip.Addr) bool {
	if p != nil && p.disk != nil {
		found, _ := p.containsContext(context.Background(), addr)
		return found
	}
	if p == nil || !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for bits := addr.BitLen(); bits >= 0; bits-- {
		if addr.Is4() {
			if bucket := p.v4[bits]; bucket != nil {
				value := addr.As4()
				if bucket.Contains(binary.BigEndian.Uint32(value[:]) >> uint(32-bits)) {
					return true
				}
			}
		} else if bucket := p.v6[bits]; len(bucket) > 0 {
			masked := netip.PrefixFrom(addr, bits).Masked().Addr().As16()
			if _, found := slices.BinarySearchFunc(bucket, masked, compareIPv6); found {
				return true
			}
		}
	}
	return false
}

func compareIPv6(a, b [16]byte) int { return bytes.Compare(a[:], b[:]) }

type prefixIndexBuilder struct {
	index prefixIndex
	v6    [129]map[[16]byte]struct{}
	limit int
}

func newPrefixIndexBuilder(limit int) *prefixIndexBuilder { return &prefixIndexBuilder{limit: limit} }

// add returns false only when a new unique prefix would exceed the limit.
func (b *prefixIndexBuilder) add(prefix netip.Prefix) bool {
	if !prefix.IsValid() {
		return false
	}
	prefix = prefix.Masked()
	bits := prefix.Bits()
	if prefix.Addr().Is4() {
		value := prefix.Addr().As4()
		// Network ordinals omit the always-zero host bits. Adjacent CIDRs then
		// become adjacent bitmap values and compress into runs without expansion.
		n := binary.BigEndian.Uint32(value[:]) >> uint(32-bits)
		bucket := b.index.v4[bits]
		if b.limit > 0 && b.index.size >= b.limit {
			return bucket != nil && bucket.Contains(n)
		}
		if bucket == nil {
			bucket = roaring.New()
			b.index.v4[bits] = bucket
		}
		if !bucket.CheckedAdd(n) {
			return true
		}
	} else {
		value := prefix.Addr().As16()
		if _, found := b.v6[bits][value]; found {
			return true
		}
		if b.limit > 0 && b.index.size >= b.limit {
			return false
		}
		if b.v6[bits] == nil {
			b.v6[bits] = map[[16]byte]struct{}{}
		}
		b.v6[bits][value] = struct{}{}
	}
	b.index.size++
	return true
}

func (b *prefixIndexBuilder) freeze() *prefixIndex {
	for _, bucket := range b.index.v4 {
		if bucket != nil {
			bucket.RunOptimize()
		}
	}
	for bits, entries := range b.v6 {
		if len(entries) == 0 {
			continue
		}
		values := make([][16]byte, 0, len(entries))
		for value := range entries {
			values = append(values, value)
		}
		slices.SortFunc(values, compareIPv6)
		b.index.v6[bits] = values
		b.v6[bits] = nil
	}
	return &b.index
}
