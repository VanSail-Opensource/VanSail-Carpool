package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/RoaringBitmap/roaring/v2"
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
func (s *Store) retryAttempt_firewallWrite(ctx context.Context, fn func(*sql.Tx) error) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
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
	return expireFirewallSnapshot(ctx, tx, table, key, generation, ordinal, participants, expired, nil)
}

// A sweep shares one immutable audience from its serializable transaction.
// Ordinary agent events still read that audience after checking their generation.
func expireFirewallSnapshot(ctx context.Context, tx *sql.Tx, table, key, generation string, ordinal uint32, participants, expired []byte, active *roaring.Bitmap) error {
	field := "prefix"
	if table == "xnode_firewall_sources" {
		field = "id"
	} else if table != "xnode_firewall_bans" {
		return errors.New("invalid firewall table")
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
	if active == nil {
		activeRaw, err := firewallAudience(ctx, tx)
		if err != nil {
			return err
		}
		active, err = firewallBitmap(activeRaw)
		if err != nil {
			return err
		}
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
