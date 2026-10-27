package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func pricingRenewalEconomicsTx(ctx context.Context, tx *sql.Tx, user, product, purchase string, nodes []string, now time.Time) (float64, float64, bool, error) {
	var raw string
	var coverage int
	err := tx.QueryRowContext(ctx, `SELECT features_json,coverage_bps FROM pricing_features WHERE entity_type='user' AND entity_id=? AND watermark>=? AND watermark<=? ORDER BY day DESC LIMIT 1`, user, pricingStamp(now.Add(-48*time.Hour)), pricingStamp(now)).Scan(&raw, &coverage)
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
	day := now.UTC().Truncate(24 * time.Hour)
	rows, err := tx.QueryContext(ctx, `WITH usage AS (SELECT u.node_id,SUM(u.upload_bytes+u.download_bytes) AS total,SUM(CASE WHEN t.purchase_id=? THEN u.upload_bytes+u.download_bytes ELSE 0 END) AS owned FROM xnode_daily_usage u LEFT JOIN xnode_periods t ON t.id=u.period_id WHERE u.date>=? AND u.date<? GROUP BY u.node_id) SELECT c.node_id,c.cycle_cost_minor,c.cycle_days,c.charge_mode,c.hourly_rate_micros,c.traffic_rate_micros,u.total,u.owned FROM pricing_cost_metadata c LEFT JOIN usage u ON u.node_id=c.node_id WHERE c.effective_at<=? ORDER BY c.node_id LIMIT 4097`, purchase, day.AddDate(0, 0, -30).Format(time.DateOnly), day.Format(time.DateOnly), pricingStamp(now))
	if err != nil {
		return 0, 0, false, err
	}
	defer rows.Close()
	type evidence struct {
		cost, cycle, hourly, traffic float64
		mode                         string
		total, owned                 sql.NullFloat64
	}
	all := map[string]evidence{}
	for rows.Next() {
		var node string
		var e evidence
		if err = rows.Scan(&node, &e.cost, &e.cycle, &e.mode, &e.hourly, &e.traffic, &e.total, &e.owned); err != nil {
			return 0, 0, false, err
		}
		if len(all) >= 4096 {
			return 0, 0, false, ErrConflict
		}
		all[node] = e
	}
	if err = rows.Err(); err != nil {
		return 0, 0, false, err
	}
	for _, node := range nodes {
		e, ok := all[node]
		if !ok || e.cycle <= 0 || !e.total.Valid || e.total.Float64 <= 0 || !e.owned.Valid || e.owned.Float64 <= 0 {
			complete = false
			continue
		}
		fixed := e.cost / e.cycle
		if e.mode == "hourly" {
			fixed = e.hourly * 24 / 10000
		}
		cost += fixed * float64(days) * min(1, e.owned.Float64/e.total.Float64)
		cost += e.owned.Float64 / 30 * float64(days) / float64(xnode.PAYGGB) * e.traffic / 10000
	}

	return cost, float64(min(feature.Renewals, 3)), complete, nil
}

// pricingCostEvidenceTx binds renewal quotes to the exact metadata versions.
func pricingCostEvidenceTx(ctx context.Context, tx *sql.Tx, nodes []string, now time.Time) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT node_id,version FROM pricing_cost_metadata WHERE effective_at<=? ORDER BY node_id LIMIT 4097`, pricingStamp(now))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	versions := map[string]int64{}
	for rows.Next() {
		var id string
		var version int64
		if err = rows.Scan(&id, &version); err != nil {
			return "", err
		}
		if len(versions) >= 4096 {
			return "", ErrConflict
		}
		versions[id] = version
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, id := range nodes {
		fmt.Fprintf(&out, "%s:%d;", id, versions[id])
	}
	return out.String(), nil
}
