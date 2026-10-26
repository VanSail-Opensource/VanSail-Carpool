package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingHealth exposes the shared memory-only telemetry projection.
func (s *Store) PricingHealth() *pricing.HealthRegistry { return s.pricingHealth }

// PricingPAYGAdmission affects only fresh future credit, never already paid bytes.
func (s *Store) retryAttempt_PricingPAYGAdmission(ctx context.Context, group, node string, now time.Time) (bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	p, err := loadPricingPolicy(ctx, tx, "payg", group, 0)
	if err != nil {
		return false, err
	}
	if err = pricingOperatorAdmissionTx(ctx, tx, "payg", group, now); err != nil {
		if err == pricing.CapacityUnsafe {
			return false, nil
		}
		return false, err
	}
	if p.DynamicEnabled && p.PurchaseMode == "DYNAMIC" {
		return s.pricingHealth.Safe([]string{node}, now), nil
	}
	return true, nil
}

func pricingNodesTx(ctx context.Context, tx *sql.Tx, kind, id string) ([]string, error) {
	query := `SELECT DISTINCT b.node_id FROM xnode_group_bindings b JOIN xnode_nodes n ON n.id=b.node_id WHERE n.enabled=1 AND b.group_id=?`
	if kind == "combo" {
		query = `SELECT DISTINCT b.node_id FROM xnode_group_bindings b JOIN xnode_nodes n ON n.id=b.node_id JOIN combos c ON c.id=? WHERE n.enabled=1 AND EXISTS(SELECT 1 FROM json_each(c.included_squad_uuids) j WHERE j.value=b.group_id)`
	}
	rows, err := tx.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var node string
		if err = rows.Scan(&node); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func pricingHealthAdmissionTx(ctx context.Context, tx *sql.Tx, kind, id string, now time.Time) error {
	nodes, err := pricingNodesTx(ctx, tx, kind, id)
	if err != nil {
		return err
	}
	if !pricing.ContextHealth(ctx).Safe(nodes, now) {
		return pricing.CapacityUnsafe
	}
	return nil
}
