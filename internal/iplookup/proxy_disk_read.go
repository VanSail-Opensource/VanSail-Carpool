package iplookup

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
)

func (p *prefixIndex) containsContext(ctx context.Context, address netip.Addr) (bool, error) {
	if p == nil {
		return false, nil
	}
	if p.disk == nil {
		return p.contains(address), nil
	}
	if !address.IsValid() {
		return false, nil
	}
	address = address.Unmap()
	candidates := make([]string, 0, address.BitLen()+1)
	args := []any{p.disk.id}
	family, _ := prefixBytes(address)
	args = append(args, family)
	familyIndex := 0
	if family == 6 {
		familyIndex = 1
	}
	for bits := 0; bits <= address.BitLen(); bits++ {
		if !p.diskBits[familyIndex][bits] {
			continue
		}
		_, network := prefixBytes(netip.PrefixFrom(address, bits).Masked().Addr())
		candidates = append(candidates, "(?,?)")
		args = append(args, bits, network)
	}
	if len(candidates) == 0 {
		return false, ctx.Err()
	}
	query := "SELECT EXISTS(SELECT 1 FROM prefixes WHERE gen=? AND family=? AND (bits,network) IN (VALUES " + strings.Join(candidates, ",") + "))"
	d := p.disk.store
	d.mu.Lock()
	defer d.mu.Unlock()
	var found bool
	err := d.db.QueryRowContext(ctx, query, args...).Scan(&found)
	return found, err
}

func countDiskIndexes(ctx context.Context, indexes []*prefixIndex) (int, error) {
	var d *inventoryDisk
	var ids []string
	var args []any
	for _, index := range indexes {
		if index == nil {
			continue
		}
		if index.disk == nil {
			return 0, fmt.Errorf("mixed inventory backends")
		}
		if d != nil && d != index.disk.store {
			return 0, fmt.Errorf("mixed inventory stores")
		}
		d = index.disk.store
		ids = append(ids, "?")
		args = append(args, index.disk.id)
	}
	if d == nil {
		return 0, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	query := "SELECT count(*) FROM (SELECT family,bits,network FROM prefixes INDEXED BY canonical WHERE gen IN (" + strings.Join(ids, ",") + ") GROUP BY family,bits,network)"
	var count int
	err := d.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func mergeDiskIndexes(ctx context.Context, previous, incoming *prefixIndex, limit int) *prefixIndex {
	if incoming == nil || incoming.count() == 0 {
		return previous.retain()
	}
	if previous == nil || previous.count() == 0 {
		return incoming
	}
	if previous.disk == nil || incoming.disk == nil {
		return previous.retain()
	}
	d := previous.disk.store
	out, err := d.newGeneration(ctx)
	if err != nil {
		return previous.retain()
	}
	writer := &diskWriter{ctx: ctx, index: out, limit: limit}
	for _, index := range []*prefixIndex{previous, incoming} {
		var family, bits int
		var network []byte
		for {
			d.mu.Lock()
			rows, e := d.db.QueryContext(ctx, "SELECT family,bits,network FROM prefixes WHERE gen=? AND (family,bits,network)>(?,?,?) ORDER BY family,bits,network LIMIT 256", index.disk.id, family, bits, network)
			if e != nil {
				d.mu.Unlock()
				writer.err = e
				break
			}
			var page []diskPrefix
			for rows.Next() {
				var value diskPrefix
				if e = rows.Scan(&value.family, &value.bits, &value.network); e != nil {
					break
				}
				page = append(page, value)
			}
			if e == nil {
				e = rows.Err()
			}
			_ = rows.Close()
			d.mu.Unlock()
			if e != nil {
				writer.err = e
				break
			}
			if len(page) == 0 {
				break
			}
			writer.pending = append(writer.pending, page...)
			writer.flush()
			if writer.err != nil {
				break
			}
			last := page[len(page)-1]
			family, bits, network = last.family, last.bits, last.network
		}
		if writer.err != nil {
			break
		}
	}
	if writer.err != nil {
		out.release()
		return previous.retain()
	}
	return out
}
