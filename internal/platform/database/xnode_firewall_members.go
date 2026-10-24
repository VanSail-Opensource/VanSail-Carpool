package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func (s *Store) ReconcileFirewallMember(ctx context.Context, node string, active, reconnect bool, observed uint64) error {
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
		var ordinal uint32
		var previous int
		err := tx.QueryRowContext(ctx, `SELECT ordinal,active FROM xnode_firewall_members WHERE node_id=?`, node).Scan(&ordinal, &previous)
		fresh := errors.Is(err, sql.ErrNoRows)
		if err != nil && !fresh {
			return err
		}
		if fresh && !active {
			return nil
		}
		if fresh {
			var id int64
			e := tx.QueryRowContext(ctx, `INSERT INTO xnode_firewall_members(node_id,active) VALUES(?,1) RETURNING ordinal`, node).Scan(&id)
			if e != nil {
				return e
			}
			if id > 4294967295 {
				return xnode.ErrInvalid
			}
			ordinal = uint32(id)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_clock SET source_version=?+1,version=?+1 WHERE singleton=1 AND version<?`, observed, observed, observed); err != nil {
			return err
		}
		if reconnect || previous != boolInt(active) {
			if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_members SET active=?,force_sync=MAX(force_sync,?),connection_epoch=connection_epoch+? WHERE node_id=?`, boolInt(active), boolInt(reconnect), boolInt(reconnect), node); err != nil {
				return err
			}
		}

		if active {
			var applied, sourceApplied, version, sourceVersion uint64
			var force int
			if err = tx.QueryRowContext(ctx, `SELECT m.applied,m.source_applied,m.force_sync,c.version,c.source_version FROM xnode_firewall_members m CROSS JOIN xnode_firewall_clock c WHERE node_id=?`, node).Scan(&applied, &sourceApplied, &force, &version, &sourceVersion); err != nil {
				return err
			}
			if force != 0 || applied < version || sourceApplied < sourceVersion {
				return queueFirewallTx(ctx, tx, node, time.Now().UTC())
			}
		}
		return nil
	})
}
func (s *Store) FirewallMember(ctx context.Context, node string) (xnode.FirewallMember, error) {
	v := xnode.FirewallMember{NodeID: node}
	var force int
	err := s.db.QueryRowContext(ctx, `SELECT m.applied,m.source_applied,m.force_sync,c.version,c.source_version,m.connection_epoch FROM xnode_firewall_members m CROSS JOIN xnode_firewall_clock c WHERE m.node_id=?`, node).Scan(&v.Applied, &v.SourceApplied, &force, &v.Version, &v.SourceVersion, &v.Epoch)
	v.Force = force != 0
	return v, err
}
func (s *Store) MarkFirewallDelivered(ctx context.Context, node string, version uint64, sources, full bool, epoch uint64) error {
	field := "applied"
	if sources {
		field = "source_applied"
	}
	force := ""
	if full && !sources {
		force = ",force_sync=0"
	}
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_firewall_members SET `+field+`=MAX(`+field+`,?)`+force+` WHERE node_id=? AND connection_epoch=?`, version, node, epoch)
	return err
}
