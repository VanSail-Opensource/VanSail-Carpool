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

// RecordXNodeTraffic commits accounting or an explicit discard before ACK.
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
		return xnode.ErrTrafficAgentMismatch
	}
	previous, previousDigest, err := lockXNodeCursorTx(ctx, tx, nodeID, agent)
	if err != nil {
		return err
	}
	if sequence <= previous {
		if sequence == previous && digest != previousDigest {
			if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, "digest_mismatch", true, report); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	// A rejected accounting batch must not leave partial usage or charges.
	if _, err = tx.ExecContext(ctx, `SAVEPOINT xnode_traffic_report`); err != nil {
		return err
	}
	err = applyXNodeTrafficTx(ctx, tx, nodeID, agent, sequence, multiplier, report)
	reason := discardedTrafficReason(err)
	discarded := reason != ""
	if err != nil && reason == "" {
		return err
	}
	if discarded {
		if _, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT xnode_traffic_report`); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `RELEASE SAVEPOINT xnode_traffic_report`); err != nil {
		return err
	}
	if discarded || sequence != previous+1 {
		if reason == "" {
			reason = "sequence_gap"
		}
		if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, reason, discarded, report); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(agent_id) DO UPDATE SET last_sequence=excluded.last_sequence,last_digest=excluded.last_digest,updated_at=excluded.updated_at`, agent, nodeID, sequence, digest, stamp(time.Now().UTC()))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func applyXNodeTrafficTx(ctx context.Context, tx *sql.Tx, nodeID, agent string, sequence uint64, multiplier int64, report wire.TrafficReport) error {
	samples := report.Samples
	var priced int64
	err := tx.QueryRowContext(ctx, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id=? AND from_sequence<=? ORDER BY from_sequence DESC LIMIT 1`, agent, sequence).Scan(&priced)
	if err == nil {
		multiplier = priced
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sampled := map[int]bool{}
	paygIDs := map[int64]bool{}
	for _, sample := range samples {
		paygIDs[int64(sample.UserID)] = true
	}
	if err = lockPAYGOwnersTx(ctx, tx, nodeID, paygIDs); err != nil {
		return err
	}
	periods := map[string]bool{}
	for _, sample := range samples {
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
			if errors.Is(err, sql.ErrNoRows) {
				return xnode.NewTrafficPeriodMissing(period)
			}
			return err
		}
	}
	for _, sample := range samples {
		sampled[sample.UserID] = true
		if handled, e := recordPAYGSampleTx(ctx, tx, nodeID, sample); e != nil {
			return e
		} else if handled {
			continue
		}
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
			if errors.Is(err, sql.ErrNoRows) {
				return xnode.ErrTrafficIdentityMissing
			}
			return err
		}
		if (!service.Valid && !migrated) || metered {
			return xnode.ErrTrafficSampleMissing
		}
	}
	return nil
}
