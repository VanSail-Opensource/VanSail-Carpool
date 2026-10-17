package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RoaringBitmap/roaring/v2"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func firewallVersion(ctx context.Context, tx *sql.Tx) (uint64, error) {
	var v uint64
	err := tx.QueryRowContext(ctx, `UPDATE xnode_firewall_clock SET version=version+1 WHERE singleton=1 RETURNING version`).Scan(&v)
	return v, err
}
func firewallBitmap(raw []byte) (*roaring.Bitmap, error) {
	b := roaring.New()
	if len(raw) > 0 {
		if _, err := b.FromBuffer(raw); err != nil {
			return nil, err
		}
		b = b.Clone()
	}
	return b, nil
}
func firewallAudience(ctx context.Context, tx *sql.Tx) ([]byte, error) {
	b := roaring.New()
	rows, err := tx.QueryContext(ctx, `SELECT ordinal FROM xnode_firewall_members WHERE active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var n uint32
		if err = rows.Scan(&n); err != nil {
			return nil, err
		}
		b.Add(n)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return b.ToBytes()
}
func queueFirewallTx(ctx context.Context, tx *sql.Tx, node string, now time.Time) error {
	for _, bank := range []string{"dynamic", "sources"} {
		raw, _ := json.Marshal(map[string]string{"nodeId": node, "bank": bank})
		// One intent per node and bank; desired state is read at execution time.
		if _, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=0,available_at=?,updated_at=? WHERE kind=? AND payload=? AND status='failed'`, stamp(now), stamp(now), xnode.FirewallJob, string(raw)); err != nil {
			return err
		}
		if err := insertOutboxTx(ctx, tx, xnode.FirewallJob, string(raw), now, now); err != nil {
			return err
		}
	}
	return nil
}
func queueFirewallFleetTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT node_id FROM xnode_firewall_members WHERE active=1`)
	if err != nil {
		return err
	}
	var nodes []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		nodes = append(nodes, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range nodes {
		if err = queueFirewallTx(ctx, tx, id, now); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) firewallWrite(ctx context.Context, fn func(*sql.Tx) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func expireFirewallParticipant(ctx context.Context, tx *sql.Tx, table, key, generation string, ordinal uint32) error {
	if table != "xnode_firewall_bans" && table != "xnode_firewall_sources" {
		return errors.New("invalid firewall table")
	}
	field := "prefix"
	if table == "xnode_firewall_sources" {
		field = "id"
	}
	var participants, expired []byte
	err := tx.QueryRowContext(ctx, `SELECT participants,expired FROM `+table+` WHERE `+field+`=? AND generation=? AND released_version=0`, key, generation).Scan(&participants, &expired)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	p, err := firewallBitmap(participants)
	if err != nil {
		return err
	}
	e, err := firewallBitmap(expired)
	if err != nil {
		return err
	}
	if ordinal != 0 && !p.Contains(ordinal) {
		return nil
	}
	if ordinal != 0 {
		e.Add(ordinal)
	}
	activeRaw, err := firewallAudience(ctx, tx)
	if err != nil {
		return err
	}
	active, err := firewallBitmap(activeRaw)
	if err != nil {
		return err
	}
	before := e.GetCardinality()
	e.Or(roaring.AndNot(p, active))
	p.And(active)
	if ordinal == 0 && before == e.GetCardinality() && !roaring.AndNot(p, e).IsEmpty() {
		return nil
	}
	raw, err := e.ToBytes()
	if err != nil {
		return err
	}
	var released uint64
	if roaring.AndNot(p, e).IsEmpty() {
		released, err = firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		if table == "xnode_firewall_sources" {
			if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_clock SET source_version=? WHERE singleton=1`, released); err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE `+table+` SET expired=?,released_version=?,released_at=CASE WHEN ?>0 THEN ? ELSE released_at END WHERE `+field+`=? AND generation=?`, raw, released, released, stamp(time.Now().UTC()), key, generation)
	if err != nil {
		return fmt.Errorf("record firewall expiry: %w", err)
	}
	return nil
}
