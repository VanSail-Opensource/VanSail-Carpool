package iplookup

import (
	"context"
	"net/netip"
)

func (p *prefixIndex) walkDiskPrefixes(ctx context.Context, send func([]string) error) error {
	d := p.disk.store
	family, bits := 0, 0
	network := []byte{}
	emitted := false
	for {
		d.mu.Lock()
		rows, err := d.db.QueryContext(ctx, `SELECT family,bits,network FROM prefixes WHERE gen=? AND (family,bits,network)>(?,?,?) ORDER BY family,bits,network LIMIT 2048`, p.disk.id, family, bits, network)
		if err != nil {
			d.mu.Unlock()
			return err
		}
		var page []string
		for rows.Next() {
			if err = rows.Scan(&family, &bits, &network); err != nil {
				break
			}
			ip, ok := netip.AddrFromSlice(network)
			if !ok {
				err = &CodeError{"IP_PROXY_INVALID_PREFIX"}
				break
			}
			page = append(page, netip.PrefixFrom(ip, bits).String())
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		d.mu.Unlock()
		if err != nil {
			return err
		}
		if len(page) == 0 {
			if !emitted {
				return send(nil)
			}
			return nil
		}
		if err = send(page); err != nil {
			return err
		}
		emitted = true
	}
}
