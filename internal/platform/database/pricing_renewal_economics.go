package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

func pricingRenewalEconomicsTx(ctx context.Context, tx *sql.Tx, user, product string, nodes []string) (float64, float64, bool, error) {
	var raw string
	var coverage int
	err := tx.QueryRowContext(ctx, `SELECT features_json,coverage_bps FROM pricing_features WHERE entity_type='user' AND entity_id=? ORDER BY day DESC LIMIT 1`, user).Scan(&raw, &coverage)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	var feature struct {
		PerNode  map[string]float64 `json:"perNodeSevenDayBytes"`
		Renewals int64              `json:"renewals30"`
	}
	if err = json.Unmarshal([]byte(raw), &feature); err != nil {
		return 0, 0, false, err
	}
	var days int
	if err = tx.QueryRowContext(ctx, `SELECT validity_days FROM combos WHERE id=?`, product).Scan(&days); err != nil {
		return 0, 0, false, err
	}
	cost := 0.0
	complete := coverage >= 8000
	for _, node := range nodes {
		var cycleCost, capacity float64
		if err = tx.QueryRowContext(ctx, `SELECT cycle_cost_minor,capacity_bytes FROM pricing_cost_metadata WHERE node_id=?`, node).Scan(&cycleCost, &capacity); errors.Is(err, sql.ErrNoRows) {
			complete = false
			continue
		} else if err != nil {
			return 0, 0, false, err
		}
		if capacity <= 0 {
			complete = false
			continue
		}
		cost += feature.PerNode[node] / 7 * float64(days) * cycleCost / capacity
	}
	return cost, float64(min(feature.Renewals, 3)), complete, nil
}
