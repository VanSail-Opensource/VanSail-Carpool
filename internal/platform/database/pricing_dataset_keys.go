package database

import (
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingDatasetTimestampKey(d pricing.Dataset, key string) bool {
	if d.Name == "operations/worker_watermark" {
		return key == "observed_at"
	}
	parts := strings.Split(d.Date, ".")
	return d.Date != "" && key == parts[len(parts)-1] && key != "day" && key != "date" && !strings.HasSuffix(key, "_ns")
}

func pricingDatasetKeyExpression(d pricing.Dataset, key string) string {
	column := "export_rows." + key
	if pricingDatasetTimestampKey(d, key) {
		return purchaseTimestampOrderSQL(column)
	}
	return column
}

func datasetKeys(d pricing.Dataset) []string {
	keys := []string{}
	if d.Date != "" {
		parts := strings.Split(d.Date, ".")
		keys = append(keys, parts[len(parts)-1])
	}
	extra := []string{"id"}
	switch d.Name {
	case "raw/node_traffic":
		extra = []string{"proxy_user_id", "node_id", "period_id"}
	case "raw/purchase_addons":
		extra = []string{"purchase_id", "group_id"}
	case "raw/payg_charges":
		extra = []string{"ledger_id"}
	case "pricing/product_settings":
		extra = []string{"product_type", "product_id"}
	case "pricing/discount_decisions":
		extra = []string{"override_id", "reference_id"}
	case "operations/worker_watermark":
		extra = []string{"observed_at"}
	case "pricing/personal_decisions":
		extra = []string{"version"}
	case "pricing/provider_costs":
		extra = []string{"node_id"}
	case "pricing/provider_cost_versions":
		extra = []string{"node_id", "version"}
	case "validation/calibration":
		extra = []string{"day", "model_version", "kind", "bucket"}
	case "validation/model_metrics":
		extra = []string{"day", "model_version", "metric", "horizon_seconds"}
	case "forecasts/historical_predictions", "forecasts/prediction_evaluations":
		extra = []string{"forecast_id"}
	default:
	}
	for _, key := range extra {
		found := false
		for _, k := range keys {
			found = found || k == key
		}
		if !found {
			keys = append(keys, key)
		}
	}
	return keys
}
func numericDatasetKey(key, dataset string) bool {
	return key == "version" && dataset != "pricing/personal_decisions" || key == "proxy_user_id" || key == "forecast_id" || key == "bucket" || key == "horizon_seconds" || strings.HasSuffix(key, "_ns")
}
