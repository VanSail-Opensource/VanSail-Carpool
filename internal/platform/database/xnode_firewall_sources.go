package database

import (
	"context"
	"database/sql"

	"time"
)

func (s *Store) FirewallRevision(ctx context.Context) (uint64, error) {
	var v uint64
	err := s.db.QueryRowContext(ctx, `SELECT inventory_version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&v)
	return v, err
}
func (s *Store) ReconcileFirewallSources(ctx context.Context, allowed []string, now time.Time) error {
	enabled := map[string]bool{}
	for _, id := range allowed {
		enabled[id] = true
	}
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM xnode_firewall_sources WHERE released_version=0`)
		if err != nil {
			return err
		}
		var remove []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			if !enabled[id] {
				remove = append(remove, id)
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if len(remove) == 0 {
			return nil
		}
		v, err := firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		for _, id := range remove {
			if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_sources SET released_version=?,released_at=? WHERE id=? AND released_version=0`, v, stamp(now), id); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_clock SET source_version=? WHERE singleton=1`, v); err != nil {
			return err
		}
		return queueFirewallFleetTx(ctx, tx, now)
	})
}
func (s *Store) SweepFirewall(ctx context.Context, now time.Time) error {
	s.firewallSweepMu.Lock()
	defer s.firewallSweepMu.Unlock()
	cursor := s.firewallSweepCursor
	prune := s.firewallRetentionAt.IsZero() || now.Before(s.firewallRetentionAt) || now.Sub(s.firewallRetentionAt) >= time.Minute
	err := s.firewallWrite(ctx, func(tx *sql.Tx) error {
		// Retry the same page if this transaction loses a serialization race.
		s.firewallSweepCursor = cursor
		var count int
		if err := s.sweepRetiredFirewallTx(ctx, tx); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_exemptions WHERE released_version=0 AND until_at<=?`, stamp(now)).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			v, err := firewallVersion(ctx, tx)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_exemptions SET released_version=? WHERE released_version=0 AND until_at<=?`, v, stamp(now)); err != nil {
				return err
			}
		}
		// Retain immutable paid grants. Operational history exceeds the five-minute
		// snapshot lifetime; lagging nodes receive a full snapshot after seven days.
		if prune {
			for _, table := range []string{"xnode_firewall_bans", "xnode_firewall_sources"} {
				if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE released_version>0 AND released_at<?`, stamp(now.Add(-7*24*time.Hour))); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		s.firewallSweepCursor = cursor
	} else if prune {
		s.firewallRetentionAt = now
	}
	return err
}
