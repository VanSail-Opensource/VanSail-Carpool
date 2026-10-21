package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const meteredProbeSelect = `SELECT proxy_user_id,period_id,operation_id,pair_key,endpoint_id,node_id,inbound_tag,valid_until,revoked_at IS NOT NULL,flushed_sequence FROM xnode_metered_probes`

func scanMeteredProbe(row rowScanner) (xnode.MeteredProbe, error) {
	var p xnode.MeteredProbe
	var until string
	var sequence sql.NullInt64
	err := row.Scan(&p.ProxyUserID, &p.PeriodID, &p.OperationID, &p.PairKey, &p.EndpointID, &p.NodeID, &p.InboundTag, &until, &p.Revoked, &sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return p, xnode.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.ValidUntil, err = time.Parse(time.RFC3339Nano, until)
	if sequence.Valid {
		value := uint64(sequence.Int64)
		p.FlushedSequence = &value
	}
	return p, err
}

func (s *Store) XNodeMeteredProbe(ctx context.Context, id int64) (xnode.MeteredProbe, error) {
	return scanMeteredProbe(s.db.QueryRowContext(ctx, meteredProbeSelect+` WHERE proxy_user_id=?`, id))
}

func (s *Store) XNodePeriodProbes(ctx context.Context, period string) ([]xnode.MeteredProbe, error) {
	rows, err := s.db.QueryContext(ctx, meteredProbeSelect+` WHERE period_id=? ORDER BY proxy_user_id`, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []xnode.MeteredProbe{}
	for rows.Next() {
		p, err := scanMeteredProbe(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// SaveXNodeMeteredProbe atomically binds access and attribution, only while the
// originating identity still owns its open period. References never rebind.
func (s *Store) SaveXNodeMeteredProbe(ctx context.Context, p xnode.MeteredProbe) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO xnode_metered_probes(proxy_user_id,period_id,operation_id,pair_key,endpoint_id,node_id,inbound_tag,valid_until) SELECT ?,?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM xnode_periods p JOIN xnode_identities i ON i.proxy_user_id=p.proxy_user_id WHERE p.id=? AND p.status='open' AND i.current_period_id=p.id AND i.access_disabled=0 AND (p.limit_bytes=0 OR p.used_bytes<p.limit_bytes))`, p.ProxyUserID, p.PeriodID, p.OperationID, p.PairKey, p.EndpointID, p.NodeID, p.InboundTag, stamp(p.ValidUntil), p.PeriodID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return xnode.ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_service_bindings(proxy_user_id,node_id,inbound_tag,valid_until) VALUES(?,?,?,?)`, p.ProxyUserID, p.NodeID, p.InboundTag, stamp(p.ValidUntil))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RevokeXNodeMeteredProbe(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_metered_probes SET revoked_at=COALESCE(revoked_at,?) WHERE proxy_user_id=?`, stamp(at), id)
	return err
}

func (s *Store) SealXNodeMeteredProbe(ctx context.Context, id int64, sequence uint64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_metered_probes SET flushed_sequence=? WHERE proxy_user_id=? AND revoked_at IS NOT NULL`, sequence, id)
	return err
}
