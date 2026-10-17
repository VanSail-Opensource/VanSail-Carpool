package database

import (
	"context"
	"database/sql"
)

func (s *Store) sweepRetiredFirewallTx(ctx context.Context, tx *sql.Tx) error {
	var retired int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_members WHERE active=0`).Scan(&retired); err != nil {
		return err
	}
	if retired == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT key,kind,target,generation FROM (
 SELECT 'b:'||generation AS key,'xnode_firewall_bans' AS kind,prefix AS target,generation FROM xnode_firewall_bans WHERE released_version=0
 UNION ALL SELECT 's:'||id,'xnode_firewall_sources',id,generation FROM xnode_firewall_sources WHERE released_version=0
 ) WHERE key>? ORDER BY key LIMIT 256`, s.firewallSweepCursor)
	if err != nil {
		return err
	}
	type item struct{ key, kind, target, gen string }
	var page []item
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.key, &v.kind, &v.target, &v.gen); err != nil {
			break
		}
		page = append(page, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range page {
		if err = expireFirewallParticipant(ctx, tx, v.kind, v.target, v.gen, 0); err != nil {
			return err
		}
	}
	if len(page) < 256 {
		s.firewallSweepCursor = ""
	} else {
		s.firewallSweepCursor = page[len(page)-1].key
	}
	return nil
}
