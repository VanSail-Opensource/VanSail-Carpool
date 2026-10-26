package database

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func personalTrafficForecasts(user, id, digest string, day, now time.Time, series map[string][]float64, results map[string]compute.Quantiles, total compute.Quantiles, totalSamples []float64) []pricing.Forecast {
	forecasts := []pricing.Forecast{}
	for node, samples := range series {
		q := results[node]
		p10, p50, p90, mean := q.P10, q.P50, q.P90, q.Mean
		for _, lead := range []int{1} {
			from := day.AddDate(0, 0, lead)
			f := pricing.Forecast{ID: id + ":" + node + ":" + from.Format("2006-01-02"), EntityType: "user_node", EntityID: user + ":" + node, Metric: "traffic_bytes", GeneratedAt: now, TargetFrom: from, TargetUntil: from.Add(24 * time.Hour), HorizonSeconds: int64(lead) * 86400, P10: p10, P50: p50, P90: p90, Mean: mean, ModelVersion: "empirical-daily-v1", Watermark: now, SnapshotHash: digest, CoverageBPS: int64(len(samples)) * 10000 / 30, HealthState: "UNKNOWN", Calibration: "UNVALIDATED"}
			forecastHash := sha256.Sum256([]byte(f.ID))
			f.ID = hex.EncodeToString(forecastHash[:16])
			if len(samples) < 7 {
				f.Calibration = "PRIOR_ONLY"
			}
			forecasts = append(forecasts, f)
		}
	}
	for lead := 1; lead <= 5 && len(totalSamples) > 0; lead++ {
		from := day.AddDate(0, 0, lead)
		key := sha256.Sum256([]byte(id + ":total:" + from.Format(time.DateOnly)))
		calibration := "UNVALIDATED"
		if len(totalSamples) < 7 {
			calibration = "PRIOR_ONLY"
		}
		forecasts = append(forecasts, pricing.Forecast{ID: hex.EncodeToString(key[:16]), EntityType: "user", EntityID: user, Metric: "traffic_bytes", GeneratedAt: now, TargetFrom: from, TargetUntil: from.Add(24 * time.Hour), HorizonSeconds: int64(lead) * 86400, P10: total.P10, P50: total.P50, P90: total.P90, Mean: total.Mean, ModelVersion: "empirical-user-total-v1", Watermark: now, SnapshotHash: digest, CoverageBPS: int64(len(totalSamples) * 10000 / 30), HealthState: "UNKNOWN", Calibration: calibration})
	}
	return forecasts
}
