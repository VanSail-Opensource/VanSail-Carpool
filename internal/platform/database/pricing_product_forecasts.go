package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingProductForecasts saves genuine daily gross receipts before refunds.
// Coupon reductions are reflected in charged amounts; costs remain separate.
func (s *Store) PricingProductForecasts(ctx context.Context, kind, id string, now time.Time) error {
	day := now.UTC().Truncate(24 * time.Hour)
	entity := kind + ":" + id
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_features WHERE entity_type='product' AND entity_id=? AND day=?)`, entity, day.Format("2006-01-02")).Scan(&exists); err != nil || exists == 1 {
		return err
	}
	query := `SELECT SUBSTR(l.created_at,1,10),-SUM(l.delta_txb_minor)` + pricingComboReceiptJoins + ` WHERE ` + pricingComboReceiptScope + ` AND l.created_at>=? AND l.created_at<? GROUP BY SUBSTR(l.created_at,1,10)`
	if kind == "payg" {
		query = `SELECT SUBSTR(l.created_at,1,10),SUM(c.blocks*c.unit_price_minor) FROM xnode_payg_charges c JOIN xnode_payg_accounts a ON a.proxy_user_id=c.proxy_user_id JOIN ledger_entries l ON l.id=c.ledger_id WHERE a.group_id=? AND l.created_at>=? AND l.created_at<? GROUP BY SUBSTR(l.created_at,1,10)`
	}
	if kind == "addon" {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, query, id, pricingStamp(day.AddDate(0, 0, -30)), pricingStamp(day))
	if err != nil {
		return err
	}
	daily := map[string]float64{}
	for rows.Next() {
		var date string
		var amount float64
		if err = rows.Scan(&date, &amount); err != nil {
			rows.Close()
			return err
		}
		daily[date] = amount
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	samples := make([]float64, 0, 30)
	for d := day.AddDate(0, 0, -30); d.Before(day); d = d.AddDate(0, 0, 1) {
		samples = append(samples, daily[d.Format("2006-01-02")])
	}
	raw, err := json.Marshal(map[string]any{"dailyGrossReceiptsMinor": daily, "definition": "charged minor units before refunds; absent transactions are genuine zero receipts", "sampleCount": 30})
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	featureID := "product:" + entity + ":" + day.Format("2006-01-02")
	q, err := s.pricingQuantiles(ctx, samples)
	if err != nil {
		return err
	}
	feature := pricingFeatureSnapshot{ID: featureID, Kind: "product", Entity: entity, Day: day.Format(time.DateOnly), At: now, Coverage: 10000, Health: "NOT_APPLICABLE", Raw: string(raw), Hash: digest}
	forecasts := []pricing.Forecast{}
	p10, p50, p90, mean := q.P10, q.P50, q.P90, q.Mean
	for _, lead := range []int{1, 7} {
		from := day.AddDate(0, 0, lead)
		idHash := sha256.Sum256([]byte(featureID + ":" + from.Format("2006-01-02")))
		f := pricing.Forecast{ID: hex.EncodeToString(idHash[:16]), EntityType: "product", EntityID: entity, Metric: "gross_revenue_minor", GeneratedAt: now, TargetFrom: from, TargetUntil: from.Add(24 * time.Hour), HorizonSeconds: int64(lead) * 86400, P10: p10, P50: p50, P90: p90, Mean: mean, ModelVersion: "empirical-receipts-v1", Watermark: now, SnapshotHash: digest, CoverageBPS: 10000, HealthState: "NOT_APPLICABLE", Calibration: "UNVALIDATED"}
		forecasts = append(forecasts, f)
	}
	return s.savePricingFeatureForecasts(ctx, feature, forecasts)
}
