package database

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// RecordXNodeTraffic commits accounting or an explicit discard before ACK.
func (s *Store) retryAttempt_RecordXNodeTraffic(ctx context.Context, nodeID, agent string, sequence uint64, digest string, report wire.TrafficReport) error {
	parsed, parseErr := wire.TrafficSequence(agent, report)
	if parseErr != nil || parsed != sequence || sequence > math.MaxInt64 || report.StreamID == "" && report.DroppedSamples != 0 {
		return xnode.ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bound string
	var multiplier int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,''),multiplier_fp FROM xnode_nodes WHERE id=$1 FOR KEY SHARE`, nodeID).Scan(&bound, &multiplier); err != nil {
		return err
	}
	if bound != agent {
		return xnode.ErrTrafficAgentMismatch
	}
	var previous uint64
	var previousDigest string
	var closed bool
	if report.StreamID == "" {
		previous, previousDigest, err = lockXNodeCursorTx(ctx, tx, nodeID, agent)
	} else {
		previous, previousDigest, closed, err = lockXNodeStreamCursorTx(ctx, tx, nodeID, agent, report.StreamID)
	}
	if err != nil {
		return err
	}
	if closed && sequence > previous {
		if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, "closed_stream", "", true, report); err != nil {
			return err
		}
		return tx.Commit()
	}
	if sequence <= previous {
		if sequence == previous && digest != previousDigest {
			if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, "digest_mismatch", "", true, report); err != nil {
				return err
			}
			if report.StreamID != "" {
				if _, err = tx.ExecContext(ctx, `UPDATE xnode_traffic_streams SET has_gaps=true WHERE agent_id=$1 AND stream_id=$2`, agent, report.StreamID); err != nil {
					return err
				}
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
	var missing interface{ MissingTrafficPeriod() string }
	missingPeriod := ""
	if errors.As(err, &missing) {
		missingPeriod = missing.MissingTrafficPeriod()
	}
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
	if discarded || sequence != previous+1 || report.DroppedSamples > 0 {
		if reason == "" {
			reason = "sequence_gap"
			if report.DroppedSamples > 0 {
				reason = "volatile_buffer_loss"
			}
		}
		if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, reason, missingPeriod, discarded, report); err != nil {
			return err
		}
	}
	if report.StreamID == "" {
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(agent_id) DO UPDATE SET last_sequence=excluded.last_sequence,last_digest=excluded.last_digest,updated_at=excluded.updated_at`, agent, nodeID, sequence, digest, stamp(time.Now().UTC()))
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE xnode_traffic_streams SET last_sequence=$1,last_digest=$2,has_gaps=has_gaps OR $3,updated_at=$4 WHERE agent_id=$5 AND stream_id=$6`, sequence, digest, discarded || sequence != previous+1 || report.DroppedSamples > 0, stamp(time.Now().UTC()), agent, report.StreamID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
