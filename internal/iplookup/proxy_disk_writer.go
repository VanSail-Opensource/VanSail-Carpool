package iplookup

import (
	"context"
	"net/netip"
	"time"
)

type diskPrefix struct {
	family, bits int
	network      []byte
}

// diskWriter owns at most 256 prefixes/hostnames between durable flushes.
type diskWriter struct {
	ctx          context.Context
	index        *prefixIndex
	pending      []diskPrefix
	domains      []string
	limit, hosts int
	err          error
}

func (w *diskWriter) add(prefix netip.Prefix) bool {
	if w.err != nil {
		return false
	}
	prefix = prefix.Masked()
	family, network := prefixBytes(prefix.Addr())
	w.pending = append(w.pending, diskPrefix{family: family, bits: prefix.Bits(), network: network})
	if len(w.pending) >= 256 {
		w.flush()
	}
	return w.err == nil
}

func prefixBytes(address netip.Addr) (int, []byte) {
	if address.Is4() {
		value := address.As4()
		return 4, value[:]
	}
	value := address.As16()
	return 6, value[:]
}

func (w *diskWriter) addDomain(host string) {
	if w.err != nil {
		return
	}
	w.domains = append(w.domains, host)
	if len(w.domains) >= 256 {
		w.flush()
	}
}

func (w *diskWriter) flush() {
	if w.err != nil || len(w.pending)+len(w.domains) == 0 {
		return
	}
	d := w.index.disk.store
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.db.BeginTx(w.ctx, nil)
	if err != nil {
		w.err = err
		return
	}
	defer func() { _ = tx.Rollback() }()
	insert, err := tx.PrepareContext(w.ctx, "INSERT OR IGNORE INTO prefixes(gen,family,bits,network) VALUES(?,?,?,?)")
	if err != nil {
		w.err = err
		return
	}
	defer insert.Close()
	count, hosts := w.index.size, w.hosts
	for _, prefix := range w.pending {
		if w.limit > 0 && count >= w.limit {
			var exists bool
			err = tx.QueryRowContext(w.ctx, "SELECT EXISTS(SELECT 1 FROM prefixes WHERE gen=? AND family=? AND bits=? AND network=?)", w.index.disk.id, prefix.family, prefix.bits, prefix.network).Scan(&exists)
			if err != nil {
				w.err = err
				return
			}
			if !exists {
				w.err = &CodeError{"IP_PROXY_LIMIT"}
				break
			}
			continue
		}
		result, e := insert.ExecContext(w.ctx, w.index.disk.id, prefix.family, prefix.bits, prefix.network)
		if e != nil {
			w.err = e
			return
		}
		n, e := result.RowsAffected()
		if e != nil {
			w.err = e
			return
		}
		count += int(n)
	}
	for _, host := range w.domains {
		if hosts >= maxProxyHosts {
			var exists bool
			if e := tx.QueryRowContext(w.ctx, "SELECT EXISTS(SELECT 1 FROM domains WHERE gen=? AND host=?)", w.index.disk.id, host).Scan(&exists); e != nil {
				w.err = e
				return
			}
			if !exists {
				w.err = &CodeError{"IP_PROXY_LIMIT"}
				break
			}
			continue
		}
		result, e := tx.ExecContext(w.ctx, "INSERT OR IGNORE INTO domains(gen,host) VALUES(?,?)", w.index.disk.id, host)
		if e != nil {
			w.err = e
			return
		}
		n, e := result.RowsAffected()
		if e != nil {
			w.err = e
			return
		}
		hosts += int(n)
	}
	if d.usage() > d.quota {
		w.err = &CodeError{"IP_PROXY_LIMIT"}
		return
	}
	if err := tx.Commit(); err != nil {
		w.err = err
		return
	}
	w.index.size, w.hosts = count, hosts
	for _, prefix := range w.pending {
		family := 0
		if prefix.family == 6 {
			family = 1
		}
		w.index.diskBits[family][prefix.bits] = true
	}
	clear(w.pending)
	clear(w.domains)
	w.pending, w.domains = w.pending[:0], w.domains[:0]
}

func (w *diskWriter) domainPage(after string) ([]string, error) {
	d := w.index.disk.store
	d.mu.Lock()
	defer d.mu.Unlock()
	rows, err := d.db.QueryContext(w.ctx, "SELECT host FROM domains WHERE gen=? AND host>? ORDER BY host LIMIT 128", w.index.disk.id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []string
	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, rows.Err()
}

func (w *diskWriter) clearDomains() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d := w.index.disk.store
	d.mu.Lock()
	defer d.mu.Unlock()
	for {
		result, err := d.db.ExecContext(ctx, "DELETE FROM domains WHERE gen=? AND host IN (SELECT host FROM domains WHERE gen=? LIMIT 256)", w.index.disk.id, w.index.disk.id)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil || n == 0 {
			return err
		}
	}
}
