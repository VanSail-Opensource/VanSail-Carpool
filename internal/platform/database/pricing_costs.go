package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingCost is administrator-owned provider cost/capacity metadata, not telemetry.
type PricingCost struct {
	NodeID      string    `json:"nodeId"`
	Cost        int64     `json:"cycleCostMinor,string"`
	CycleDays   int64     `json:"cycleDays"`
	Capacity    int64     `json:"capacityBytes,string"`
	EffectiveAt time.Time `json:"effectiveAt"`
	Version     int64     `json:"version"`
	Reason      string    `json:"reason"`
	ActualPaid  *int64    `json:"actualPaidMinor,string,omitempty"`
}

func (s *Store) retryAttempt_SavePricingCost(ctx context.Context, actor string, c PricingCost, now time.Time) error {
	if c.ActualPaid != nil && (*c.ActualPaid < 0 || *c.ActualPaid > pricing.MaxPrice) {
		return ErrConflict
	}
	if c.Cost < 0 || c.Cost > pricing.MaxPrice || c.CycleDays < 1 || c.CycleDays > 366 || c.Capacity <= 0 || c.Reason == "" || len(c.Reason) > 1000 || c.EffectiveAt.IsZero() || c.EffectiveAt.After(now) {
		return ErrConflict
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
	if err = pricingAdminTx(ctx, tx, actor); err != nil {
		return err
	}
	var result sql.Result
	if c.Version == 0 {
		result, err = tx.ExecContext(ctx, `INSERT INTO pricing_cost_metadata(node_id,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,updated_by,updated_at,version) VALUES(?,?,?,?,?,?,?,1) ON CONFLICT(node_id) DO NOTHING`, c.NodeID, c.Cost, c.CycleDays, c.Capacity, pricingStamp(c.EffectiveAt), actor, pricingStamp(now))
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE pricing_cost_metadata SET cycle_cost_minor=?,cycle_days=?,capacity_bytes=?,effective_at=?,updated_by=?,updated_at=?,version=version+1 WHERE node_id=? AND version=?`, c.Cost, c.CycleDays, c.Capacity, pricingStamp(c.EffectiveAt), actor, pricingStamp(now), c.NodeID, c.Version)
	}
	if err != nil {
		return err
	}
	if n, e := result.RowsAffected(); e != nil {
		return e
	} else if n != 1 {
		return pricing.VersionConflict
	}
	if err = pricingAuditTx(ctx, tx, actor, "cost.save", c.NodeID, c.Reason, c, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO pricing_cost_versions(node_id,version,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,recorded_at,actor_id,reason,actual_paid_minor) VALUES(?,?,?,?,?,?,?,?,?,?)`, c.NodeID, c.Version+1, c.Cost, c.CycleDays, c.Capacity, pricingStamp(c.EffectiveAt), pricingStamp(now), actor, c.Reason, c.ActualPaid); err != nil {
		return err
	}
	if err = pricingChangeNoticeTx(ctx, tx, actor, "COST", c.NodeID, c.Reason, now); err != nil {
		return err
	}
	// Invalidate future analytical offers for products fulfilled by this machine.
	// Historical publications, charges and contracts remain immutable.
	if _, err = tx.ExecContext(ctx, `UPDATE pricing_product_settings SET version=version+1,updated_at=?,updated_by=? WHERE dynamic_enabled=1 AND ((product_type IN ('addon','payg') AND EXISTS(SELECT 1 FROM xnode_group_bindings b WHERE b.group_id=pricing_product_settings.product_id AND b.node_id=?)) OR (product_type='combo' AND EXISTS(SELECT 1 FROM combos c JOIN json_each(c.included_squad_uuids) j ON 1=1 JOIN xnode_group_bindings b ON b.group_id=j.value WHERE c.id=pricing_product_settings.product_id AND b.node_id=?)))`, pricingStamp(now), actor, c.NodeID, c.NodeID); err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		s.pricingCache.Clear()
		s.pricingExposureCache.Clear()
	}
	return err
}
