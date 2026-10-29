package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// pricingProductReadiness batches publication and completed-calculation evidence.
// It is an administrative projection; checkout still resolves inside settlement.
func (s *Store) pricingProductReadiness(ctx context.Context, products []map[string]any, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT p.product_type,p.product_id,d.state,d.generated_at,COALESCE(m.price_minor,v.price_minor),COALESCE(m.valid_until,v.valid_until),COALESCE(json_extract(d.trace_json,'$.reason'),''),(m.id IS NOT NULL) FROM pricing_product_settings p LEFT JOIN pricing_decisions d ON d.id=(SELECT x.id FROM pricing_decisions x WHERE x.product_type=p.product_type AND x.product_id=p.product_id AND x.policy_version>=p.version ORDER BY x.generated_at DESC LIMIT 1) LEFT JOIN pricing_published_prices v ON v.id=p.last_approved_price_version AND v.valid_from<=? LEFT JOIN pricing_published_prices m ON m.id=(SELECT x.id FROM pricing_published_prices x WHERE x.product_type=p.product_type AND x.product_id=p.product_id AND x.policy_version=p.version AND x.source='MANUAL' AND x.valid_from<=? AND x.valid_until>? AND EXISTS(SELECT 1 FROM pricing_admin_overrides o WHERE o.id=x.id AND o.status='ACTIVE') ORDER BY x.created_at DESC LIMIT 1) WHERE p.dynamic_enabled=1 AND p.purchase_pricing_mode='DYNAMIC' ORDER BY p.product_type,p.product_id LIMIT 4097`, pricingStamp(now), pricingStamp(now), pricingStamp(now))
	if err != nil {
		return err
	}
	defer rows.Close()
	type evidence struct {
		state, at, until sql.NullString
		price            sql.NullInt64
		reason           string
		manual           bool
	}
	all := map[string]evidence{}
	for rows.Next() {
		var kind, id string
		var e evidence
		if err = rows.Scan(&kind, &id, &e.state, &e.at, &e.price, &e.until, &e.reason, &e.manual); err != nil {
			return err
		}
		if len(all) >= 4096 {
			return pricing.WorkerResourceLimit
		}
		all[kind+":"+id] = e
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range products {
		p := item["policy"].(pricing.Policy)
		item["pricingStatus"], item["currentPriceMinor"] = "FIXED", item["fixedPriceMinor"]
		if !p.DynamicEnabled || p.PurchaseMode != "DYNAMIC" {
			continue
		}
		e := all[p.ProductType+":"+p.ProductID]
		item["pricingStatus"], item["currentPriceMinor"] = "PENDING", nil
		if e.at.Valid {
			item["calculatedAt"] = e.at.String
			item["pricingReason"] = e.reason
		}
		if e.price.Valid {
			until, parseErr := parseStamp(e.until.String)
			if parseErr != nil {
				return parseErr
			}
			if !now.Before(until) {
				item["pricingStatus"] = "STALE"
				continue
			}
			// Calculation cadence is not the published price's expiry. The same
			// approved price remains usable while another calculation is pending.
			item["currentPriceValidUntil"] = pricingStamp(until)
			bounded, boundErr := pricing.BoundPrice(p.FixedPrice, e.price.Int64, p.MaxIncreaseBPS, p.MaxDiscountBPS)
			if !e.manual && (boundErr != nil || bounded != e.price.Int64) {
				item["pricingStatus"] = "UNAVAILABLE"
				continue
			}
			item["currentPriceMinor"] = strconv.FormatInt(e.price.Int64, 10)
		} else if p.LastApprovedVersion == "" {
			item["currentPriceMinor"] = item["fixedPriceMinor"]
		} else {
			item["pricingStatus"] = "UNAVAILABLE"
			continue
		}
		if e.at.Valid {
			item["pricingStatus"] = "UNCHANGED"
		}
		if e.manual || e.state.String == "PUBLISHED" {
			item["pricingStatus"] = "PUBLISHED"
		}
	}
	return nil
}
