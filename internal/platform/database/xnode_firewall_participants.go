package database

import (
	"context"
	"database/sql"
	"errors"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Join only the bounded page about to be delivered. New nodes do not rewrite
// the entire fleet inventory in their WebSocket handshake transaction.
func (s *Store) JoinFirewallParticipants(ctx context.Context, node string, bans []wire.FirewallBan, sources []wire.FirewallSource) error {
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
		var ordinal uint32
		if err := tx.QueryRowContext(ctx, `SELECT ordinal FROM xnode_firewall_members WHERE node_id=? AND active=1`, node).Scan(&ordinal); err != nil {
			return err
		}
		join := func(table, key, gen string) error {
			field := "prefix"
			if table == "xnode_firewall_sources" {
				field = "id"
			}
			var raw []byte
			err := tx.QueryRowContext(ctx, `SELECT participants FROM `+table+` WHERE `+field+`=? AND generation=? AND released_version=0`, key, gen).Scan(&raw)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			p, err := firewallBitmap(raw)
			if err != nil {
				return err
			}
			if p.Contains(ordinal) {
				return nil
			}
			p.Add(ordinal)
			raw, err = p.ToBytes()
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `UPDATE `+table+` SET participants=? WHERE `+field+`=? AND generation=?`, raw, key, gen)
			return err
		}
		for _, b := range bans {
			if err := join("xnode_firewall_bans", b.Prefix, b.Generation); err != nil {
				return err
			}
		}
		for _, source := range sources {
			if err := join("xnode_firewall_sources", source.ID, source.Generation); err != nil {
				return err
			}
		}
		return nil
	})
}
