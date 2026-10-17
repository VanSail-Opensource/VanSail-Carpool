package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/RoaringBitmap/roaring/v2"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"time"
)

func (s *Store) PublishFirewallSource(ctx context.Context, source wire.FirewallSource, now time.Time) (bool, error) {
	active := false
	err := s.firewallWrite(ctx, func(tx *sql.Tx) error {
		var released uint64
		err := tx.QueryRowContext(ctx, `SELECT released_version FROM xnode_firewall_sources WHERE id=? AND generation=?`, source.ID, source.Generation).Scan(&released)
		if err == nil {
			active = released == 0
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		version, err := firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_clock SET source_version=? WHERE singleton=1`, version); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_sources SET released_version=?,released_at=? WHERE id=? AND released_version=0`, version, stamp(now), source.ID); err != nil {
			return err
		}
		audience, err := firewallAudience(ctx, tx)
		if err != nil {
			return err
		}
		empty, err := roaring.New().ToBytes()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_firewall_sources(id,generation,until_at,refresh_seconds,issued_version,participants,expired) VALUES(?,?,?,?,?,?,?)`, source.ID, source.Generation, stamp(source.Until), source.RefreshSeconds, version, audience, empty); err != nil {
			return err
		}
		active = true
		return queueFirewallFleetTx(ctx, tx, now)
	})
	return active, err
}

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
func (s *Store) FirewallSources(ctx context.Context) ([]wire.FirewallSource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,generation,until_at,refresh_seconds FROM xnode_firewall_sources WHERE released_version=0 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []wire.FirewallSource
	for rows.Next() {
		var v wire.FirewallSource
		var until string
		if err = rows.Scan(&v.ID, &v.Generation, &until, &v.RefreshSeconds); err != nil {
			return nil, err
		}
		if v.Until, err = time.Parse(time.RFC3339Nano, until); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *Store) SweepFirewall(ctx context.Context, now time.Time) error {
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
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
		for _, table := range []string{"xnode_firewall_bans", "xnode_firewall_sources"} {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE released_version>0 AND released_at<?`, stamp(now.Add(-7*24*time.Hour))); err != nil {
				return err
			}
		}
		return nil
	})
}
