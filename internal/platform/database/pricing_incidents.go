package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingMaintenance reads explicit bounded administrative maintenance declarations.
func (s *Store) PricingMaintenance(ctx context.Context, node string, now time.Time) (bool, error) {
	var yes int
	err := s.db.QueryRowContext(ctx, "SELECT (EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE command='MAINTENANCE' AND status='ACTIVE' AND starts_at<=$1 AND expires_at>$2 AND (scope_type='global' OR (scope_type IN ('node','machine') AND scope_id=$3))))::integer", pricingStamp(now), pricingStamp(now), node).Scan(&yes)
	return yes == 1, err
}

// RecordPricingHealth retains incident transitions, never machine samples or live rates.
func (s *Store) retryAttempt_RecordPricingHealth(ctx context.Context, h pricing.Health) error {
	// Every recorder locks the same durable node before reading incidents. Read
	// committed gives a waiter a fresh incident snapshot after acquiring the lock.
	// The lock also excludes node deletion without retaining live metric samples.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var nodeID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM xnode_nodes WHERE id=$1 FOR UPDATE`, h.NodeID).Scan(&nodeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // Stale telemetry for a removed node cannot create history.
		}
		return err
	}
	var started, ended sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT MAX(started_at),MAX(ended_at) FROM pricing_operational_incidents
		WHERE scope_type='node' AND scope_id=$1`, nodeID).Scan(&started, &ended); err != nil {
		return err
	}
	for _, latest := range []sql.NullString{started, ended} {
		if !latest.Valid {
			continue
		}
		at, err := parseStamp(latest.String)
		if err != nil {
			return err
		}
		if !h.At.After(at) {
			return nil // Delayed samples cannot reverse a retained newer transition.
		}
	}
	var id, state string
	err = tx.QueryRowContext(ctx, `SELECT id,state FROM pricing_operational_incidents WHERE scope_type='node'
		AND scope_id=$1 AND ended_at IS NULL ORDER BY started_at DESC,id DESC LIMIT 1`, nodeID).Scan(&id, &state)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	keep := ""
	if state == h.State {
		keep = id
	}
	// Heal duplicate open rows left by older concurrent recorders as well as the
	// usual previous state, so a recovered node cannot retain an orphan censor.
	if _, err = tx.ExecContext(ctx, `UPDATE pricing_operational_incidents SET ended_at=$1 WHERE scope_type='node'
		AND scope_id=$2 AND ended_at IS NULL AND id<>$3`, pricingStamp(h.At), nodeID, keep); err != nil {
		return err
	}
	if h.State != "HEALTHY" && keep == "" {
		id, err = ids.New()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO pricing_operational_incidents(id,scope_type,scope_id,state,started_at,reason) VALUES($1,'node',$2,$3,$4,$5)`, id, h.NodeID, h.State, pricingStamp(h.At), "authenticated telemetry health transition")
		if err != nil {
			return err
		}
		if h.State == "OFFLINE" || h.State == "OVERLOADED" || h.State == "TELEMETRY_STALE" {
			raw, e := json.Marshal(map[string]string{"incidentId": id})
			if e != nil {
				return e
			}
			if e = insertOutboxTx(ctx, tx, "pricing_health_notice", string(raw), h.At.Add(time.Minute), h.At); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
