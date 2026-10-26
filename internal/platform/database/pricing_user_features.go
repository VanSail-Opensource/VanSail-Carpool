package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingUserFeatureBatch retains compact actual per-node histories for relevant
// active members; it never expands every possible user/node/product combination.
func (s *Store) PricingUserFeatureBatch(ctx context.Context, now time.Time, cursor string) (string, error) {
	release, slotErr := s.acquirePricingCompute()
	if slotErr != nil {
		return cursor, slotErr
	}
	defer release()
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT COALESCE(i.user_id,a.user_id) AS owner FROM xnode_daily_usage d JOIN xnode_identities i ON i.proxy_user_id=d.proxy_user_id LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=d.proxy_user_id WHERE d.date>=? AND COALESCE(i.user_id,a.user_id)>? AND NOT EXISTS(SELECT 1 FROM pricing_features f WHERE f.entity_type='user' AND f.entity_id=COALESCE(i.user_id,a.user_id) AND f.day=?) ORDER BY owner LIMIT 16`, now.AddDate(0, 0, -30).Format("2006-01-02"), cursor, now.UTC().Format("2006-01-02"))
	if err != nil {
		return cursor, err
	}
	users := []string{}
	for rows.Next() {
		var user string
		if err = rows.Scan(&user); err != nil {
			rows.Close()
			return cursor, err
		}
		users = append(users, user)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return cursor, err
	}
	for _, user := range users {
		if err = s.pricingUserFeatures(ctx, user, now); err != nil {
			if errors.Is(err, ErrPricingComputePending) {
				continue
			}
			return cursor, err
		}
	}
	if len(users) < 16 {
		return "", nil
	}
	return users[len(users)-1], nil
}

func (s *Store) pricingUserFeatures(ctx context.Context, user string, now time.Time) error {
	day := now.UTC().Truncate(24 * time.Hour)
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_features WHERE entity_type='user' AND entity_id=? AND day=?)`, user, day.Format("2006-01-02")).Scan(&exists); err != nil || exists == 1 {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT d.node_id,d.date,SUM(d.upload_bytes+d.download_bytes) FROM xnode_daily_usage d JOIN xnode_identities i ON i.proxy_user_id=d.proxy_user_id LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=d.proxy_user_id WHERE COALESCE(i.user_id,a.user_id)=? AND d.date>=? AND d.date<? GROUP BY d.node_id,d.date ORDER BY d.node_id,d.date LIMIT 32769`, user, day.AddDate(0, 0, -30).Format("2006-01-02"), day.Format("2006-01-02"))
	if err != nil {
		return err
	}
	series := map[string][]float64{}
	perNode := map[string]float64{}
	dates := map[string]bool{}
	dailyTotals := map[string]float64{}
	dailyNodes := map[string]int{}
	observed := 0
	for rows.Next() {
		observed++
		if observed > 32768 {
			rows.Close()
			return pricing.StoragePressure
		}
		var node, date string
		var bytes float64
		if err = rows.Scan(&node, &date, &bytes); err != nil {
			rows.Close()
			return err
		}
		series[node] = append(series[node], bytes)
		dates[date] = true
		dailyTotals[date] += bytes
		dailyNodes[date]++
		if date >= day.AddDate(0, 0, -7).Format("2006-01-02") {
			perNode[node] += bytes
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var renewals int64
	if err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN kind='automatic_renewal' THEN 1 ELSE 0 END),0) FROM ledger_entries WHERE user_id=? AND created_at>=?`, user, pricingStamp(day.AddDate(0, 0, -30))).Scan(&renewals); err != nil {
		return err
	}
	sourceRaw, e := json.Marshal(series)
	if e != nil {
		return e
	}
	sourceHash := sha256.Sum256(sourceRaw)
	distributions := map[string]any{}
	results := map[string]compute.Quantiles{}
	pending := false
	for node, samples := range series {
		q, e := s.pricingQuantiles(ctx, samples)
		if errors.Is(e, ErrPricingComputePending) {
			pending = true
			continue
		}
		if e != nil {
			return e
		}
		results[node] = q
		p10, p50, p90, mean := q.P10, q.P50, q.P90, q.Mean
		distributions[node] = map[string]any{"p10": p10, "p50": p50, "p90": p90, "mean": mean, "samples": len(samples)}
	}
	totalSamples := []float64{}
	for d := day.AddDate(0, 0, -30); d.Before(day); d = d.AddDate(0, 0, 1) {
		if value, ok := dailyTotals[d.Format(time.DateOnly)]; ok && dailyNodes[d.Format(time.DateOnly)] == len(series) {
			totalSamples = append(totalSamples, value)
		}
	}
	total, err := s.pricingQuantiles(ctx, totalSamples)
	if errors.Is(err, ErrPricingComputePending) {
		pending = true
	} else if err != nil {
		return err
	}
	if pending {
		return ErrPricingComputePending
	}
	raw, err := json.Marshal(map[string]any{"perNodeSevenDayBytes": perNode, "totalDistribution": total, "dailyObservedDays": len(totalSamples), "observedNodeDays": observed, "expectedNodeDays": len(series) * 30, "nodeDistributions": distributions, "sourceSeriesHash": hex.EncodeToString(sourceHash[:]), "activeDays30": len(dates), "renewals30": renewals, "definition": "observed native usage; absent days are unavailable"})
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	id := "user:" + user + ":" + day.Format("2006-01-02")
	feature := pricingFeatureSnapshot{ID: id, Kind: "user", Entity: user, Day: day.Format(time.DateOnly), At: now, Coverage: int64(len(dates) * 10000 / 30), Health: "UNKNOWN", Raw: string(raw), Hash: digest}
	forecasts := personalTrafficForecasts(user, id, digest, day, now, series, results, total, totalSamples)
	return s.savePricingFeatureForecasts(ctx, feature, forecasts)
}
