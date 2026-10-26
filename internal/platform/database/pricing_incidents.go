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
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE command='MAINTENANCE' AND status='ACTIVE' AND starts_at<=? AND expires_at>? AND (scope_type='global' OR (scope_type IN ('node','machine') AND scope_id=?)))`, pricingStamp(now), pricingStamp(now), node).Scan(&yes)
	return yes == 1, err
}

// RecordPricingHealth retains incident transitions, never machine samples or live rates.
func (s *Store) RecordPricingHealth(ctx context.Context, h pricing.Health) error {
	var id, state string
	err := s.db.QueryRowContext(ctx, `SELECT id,state FROM pricing_operational_incidents WHERE scope_type='node' AND scope_id=? AND ended_at IS NULL ORDER BY started_at DESC LIMIT 1`, h.NodeID).Scan(&id, &state)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if state == h.State || id == "" && h.State == "HEALTHY" {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if id != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE pricing_operational_incidents SET ended_at=? WHERE id=? AND ended_at IS NULL`, pricingStamp(h.At), id); err != nil {
			return err
		}
	}
	if h.State != "HEALTHY" {
		id, err = ids.New()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO pricing_operational_incidents(id,scope_type,scope_id,state,started_at,reason) VALUES(?,'node',?,?,?,?)`, id, h.NodeID, h.State, pricingStamp(h.At), "authenticated telemetry health transition")
		if err != nil {
			return err
		}
		if h.State == "OFFLINE" || h.State == "OVERLOADED" || h.State == "TELEMETRY_STALE" {
			raw, e := json.Marshal(map[string]string{"incidentId": id})
			if e != nil {
				return e
			}
			if e = insertOutboxTx(ctx, tx, "pricing_health_notice", string(raw), h.At, h.At); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
