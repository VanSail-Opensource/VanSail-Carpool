package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// RecordXNodeTraffic commits one contiguous immutable journal sequence before ACK.
func (s *Store) retryAttempt_RecordXNodeTraffic(ctx context.Context, nodeID, agent string, sequence uint64, digest string, report wire.TrafficReport) error {
	if sequence == 0 || sequence > math.MaxInt64 || report.ID != agent+":"+strconv.FormatUint(sequence, 10) {
		return xnode.ErrInvalid
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bound string
	var multiplier int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,''),multiplier_fp FROM xnode_nodes WHERE id=?`, nodeID).Scan(&bound, &multiplier); err != nil {
		return err
	}
	if bound != agent {
		return xnode.ErrConflict
	}
	var previous uint64
	var previousDigest string
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES(?,?,0,'',?) ON CONFLICT(agent_id) DO NOTHING`, agent, nodeID, stamp(time.Now().UTC())); err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `SELECT last_sequence,last_digest FROM xnode_report_cursors WHERE agent_id=? /*pg:FOR UPDATE*/`, agent).Scan(&previous, &previousDigest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if sequence <= previous {
		if sequence == previous && digest != previousDigest {
			return xnode.ErrConflict
		}
		return tx.Commit()
	}
	if sequence != previous+1 {
		return xnode.ErrConflict
	}
	var priced int64
	err = tx.QueryRowContext(ctx, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id=? AND from_sequence<=? ORDER BY from_sequence DESC LIMIT 1`, agent, sequence).Scan(&priced)
	if err == nil {
		multiplier = priced
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sampled := map[int]bool{}
	periods := map[string]bool{}
	for _, sample := range report.Samples {
		periods[sample.AccountingPeriod] = true
	}
	ordered := make([]string, 0, len(periods))
	for period := range periods {
		ordered = append(ordered, period)
	}
	sort.Strings(ordered)
	for _, period := range ordered {
		var locked string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM xnode_periods WHERE id=? /*pg:FOR UPDATE*/`, period).Scan(&locked); err != nil {
			return err
		}
	}
	for _, sample := range report.Samples {
		sampled[sample.UserID] = true
		if err = recordXNodeSample(ctx, tx, nodeID, multiplier, sample); err != nil {
			return err
		}
	}
	for user, bytes := range report.Traffic {
		if sampled[user] || bytes == [2]uint64{} {
			continue
		}
		var service sql.NullString
		var metered, migrated bool
		if err = tx.QueryRowContext(ctx, `SELECT service_name,
			EXISTS(SELECT 1 FROM xnode_metered_probes p WHERE p.proxy_user_id=i.proxy_user_id),
			EXISTS(SELECT 1 FROM member_identity_aliases a WHERE (a.old_proxy_id=? OR a.proxy_id=?) AND a.proxy_id=i.proxy_user_id AND a.user_id=i.user_id)
			FROM xnode_identities i WHERE proxy_user_id=COALESCE((SELECT proxy_id FROM member_identity_aliases WHERE old_proxy_id=?),?)`, user, user, user, user).Scan(&service, &metered, &migrated); err != nil {
			return err
		}
		if (!service.Valid && !migrated) || metered {
			return xnode.ErrConflict
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(agent_id) DO UPDATE SET last_sequence=excluded.last_sequence,last_digest=excluded.last_digest,updated_at=excluded.updated_at`, agent, nodeID, sequence, digest, stamp(time.Now().UTC()))
	if err != nil {
		return err
	}
	return tx.Commit()
}
