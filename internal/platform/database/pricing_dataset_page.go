package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	"strconv"
	"strings"
	"time"
)

type PricingDatasetPage struct {
	Items      []map[string]any `json:"items"`
	Limit      int              `json:"limit"`
	Truncated  bool             `json:"truncated"`
	NextCursor string           `json:"nextCursor"`
}
type pricingDatasetCursor struct {
	Scope  string   `json:"s"`
	Values []string `json:"v"`
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

func (s *Store) ReadPricingDatasetPage(ctx context.Context, name string, scope PricingExportScope, limit int, cursor string) (PricingDatasetPage, error) {
	key, _ := json.Marshal([]any{name, scope, limit, cursor})
	return pricingCached(ctx, s, "dataset-page:"+string(key), 20*time.Second, func() (PricingDatasetPage, error) { return s.pricingDatasetPage(ctx, name, scope, limit, cursor) })
}
func (s *Store) pricingDatasetPage(ctx context.Context, name string, scope PricingExportScope, limit int, cursor string) (PricingDatasetPage, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	page := PricingDatasetPage{Items: []map[string]any{}, Limit: limit}
	var source *pricing.Dataset
	for _, d := range pricing.Datasets() {
		if d.Name == name {
			v := d
			source = &v
			break
		}
	}
	if source == nil {
		return page, ErrNotFound
	}
	keys := datasetKeys(*source)
	raw, _ := json.Marshal([]any{name, scope, keys})
	binding := compute.Digest(raw)
	query, args := pricingDatasetQuery(*source, scope)
	query = "SELECT * FROM (" + query + ") export_rows"
	if cursor != "" {
		if len(cursor) > 8192 {
			return page, ErrConflict
		}
		encoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return page, ErrConflict
		}
		var c pricingDatasetCursor
		if json.Unmarshal(encoded, &c) != nil || c.Scope != binding || len(c.Values) != len(keys) {
			return page, ErrConflict
		}
		values := make([]any, len(keys))
		for i, key := range keys {
			if numericDatasetKey(key, name) {
				n, e := strconv.ParseInt(c.Values[i], 10, 64)
				if e != nil {
					return page, ErrConflict
				}
				values[i] = n
			} else {
				values[i] = c.Values[i]
			}
		}
		alternatives := []string{}
		for i, key := range keys {
			parts := []string{}
			for j := 0; j < i; j++ {
				parts = append(parts, "export_rows."+keys[j]+"=?")
				args = append(args, values[j])
			}
			parts = append(parts, "export_rows."+key+"<?")
			args = append(args, values[i])
			alternatives = append(alternatives, "("+strings.Join(parts, " AND ")+")")
		}
		query += " WHERE "
		query += "(" + strings.Join(alternatives, " OR ") + ")"
	}
	ordering := []string{}
	for _, key := range keys {
		ordering = append(ordering, "export_rows."+key+" DESC")
	}
	query += " ORDER BY " + strings.Join(ordering, ",") + " LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return page, err
	}
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err = rows.Scan(pointers...); err != nil {
			return page, err
		}
		record := map[string]any{}
		for i, column := range columns {
			switch v := values[i].(type) {
			case int64:
				record[column] = strconv.FormatInt(v, 10)
			case []byte:
				record[column] = string(v)
			default:
				record[column] = v
			}
		}
		page.Items = append(page.Items, record)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.Truncated = true
		c := pricingDatasetCursor{Scope: binding, Values: []string{}}
		for _, key := range keys {
			value, ok := page.Items[limit-1][key].(string)
			if !ok {
				return page, ErrConflict
			}
			c.Values = append(c.Values, value)
		}
		encoded, err := json.Marshal(c)
		if err != nil {
			return page, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return page, nil
}
