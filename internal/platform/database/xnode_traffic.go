package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"math"
)

// RecordXNodeTraffic commits accounting or an explicit discard before ACK.
func (s *Store) retryAttempt_RecordXNodeTraffic(ctx context.Context, nodeID, agent string, sequence uint64, digest string, report wire.TrafficReport) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.recordXNodeTrafficTx(ctx, tx, nodeID, agent, sequence, digest, report, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) recordXNodeTrafficTx(ctx context.Context, tx *sql.Tx, nodeID, agent string, sequence uint64, digest string, report wire.TrafficReport, batch *trafficBatchState) error {
	parsed, parseErr := wire.TrafficSequence(agent, report)
	if parseErr != nil || parsed != sequence || sequence > math.MaxInt64 || report.StreamID == "" && (report.DroppedSamples != 0 || report.DroppedHourlySamples != 0 || len(report.HourlySamples) > 0) {
		return xnode.ErrInvalid
	}

	bound, multiplier, err := trafficNodeTx(ctx, tx, batch, nodeID)
	if err != nil {
		return err
	}
	if bound != agent {
		return xnode.ErrTrafficAgentMismatch
	}
	key := trafficCursorKey{agent, report.StreamID}
	cursor, err := trafficCursorTx(ctx, tx, batch, nodeID, key)
	if err != nil {
		return err
	}
	previous, previousDigest, closed := cursor.sequence, cursor.digest, cursor.closed
	if closed && sequence > previous {
		if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, "closed_stream", "", true, report); err != nil {
			return err
		}
		return nil
	}
	if sequence <= previous {
		if sequence == previous && digest != previousDigest {
			if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, "digest_mismatch", "", true, report); err != nil {
				return err
			}
			if report.StreamID != "" {
				if err = storeTrafficCursorTx(ctx, tx, batch, nodeID, key, previous, previousDigest, true); err != nil {
					return err
				}
			}
		}
		return nil
	}
	// A rejected accounting batch must not leave partial usage or charges.
	if _, err = tx.ExecContext(ctx, `SAVEPOINT xnode_traffic_report`); err != nil {
		return err
	}
	var native *nativeReportBuffer
	if batch != nil {
		native = batch.native.fork()
	}
	err = applyXNodeTrafficTx(ctx, tx, nodeID, agent, sequence, multiplier, report, batch, native)
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
	if discarded || sequence != previous+1 || report.DroppedSamples > 0 || report.DroppedHourlySamples > 0 {
		if reason == "" {
			reason = "sequence_gap"
			if report.DroppedSamples > 0 {
				reason = "volatile_buffer_loss"
			} else if report.DroppedHourlySamples > 0 {
				reason = "volatile_hourly_history_loss"
			}
		}
		if err = recordDiscardedTrafficTx(ctx, tx, nodeID, agent, sequence, previous, digest, reason, missingPeriod, discarded, report); err != nil {
			return err
		}
	}
	if err = storeTrafficCursorTx(ctx, tx, batch, nodeID, key, sequence, digest, discarded || sequence != previous+1 || report.DroppedSamples > 0 || report.DroppedHourlySamples > 0); err != nil {
		return err
	}
	if err = startUsageHistoryTx(ctx, tx); err != nil {
		return err
	}
	if native != nil && !discarded {
		native.accept()
	}

	return nil
}
