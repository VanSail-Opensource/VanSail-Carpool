package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// RecordPricingExposures records shown offers, never synthetic training samples.
func (s *Store) RecordPricingExposures(ctx context.Context, user string, catalog model.Catalog, now time.Time) error {
	shown := []struct {
		ID      string
		Price   int64
		Version string
	}{}
	for i, c := range catalog.Combos {
		if i >= 64 {
			break
		}
		shown = append(shown, struct {
			ID      string
			Price   int64
			Version string
		}{c.ID, c.PriceTXBMinor, func() string {
			if c.PricingDecision != nil {
				return c.PricingDecision.Version
			}
			return ""
		}()})
	}
	raw, _ := json.Marshal(shown)
	fingerprint := sha256.Sum256(raw)
	_, err := s.pricingExposureCache.Load(ctx, "catalog:"+user+":"+hex.EncodeToString(fingerprint[:])+":"+now.UTC().Format("2006-01-02"), 24*time.Hour, func() ([]byte, error) {
		rows, e := s.db.QueryContext(ctx, `SELECT product_id FROM pricing_product_settings WHERE product_type='combo' AND dynamic_enabled=1 LIMIT 512`)
		if e != nil {
			return nil, e
		}
		enabled := map[string]bool{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			enabled[id] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		for i, c := range catalog.Combos {
			if i >= 64 {
				break
			}
			if !enabled[c.ID] {
				continue
			}
			if err := s.recordPricingOffer(ctx, user, "combo", c.ID, c.PriceTXBMinor, now); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err == nil {
		for _, p := range catalog.Addons {
			if err = s.RecordPricingOffer(ctx, user, "addon", p.GroupID, p.PriceTXBMinor, now); err != nil {
				return err
			}
		}
	}
	return err
}

func (s *Store) RecordPricingOffer(ctx context.Context, user, kind, product string, price int64, now time.Time) error {
	return s.recordPricingOffer(ctx, user, kind, product, price, now)
}
func (s *Store) recordPricingOffer(ctx context.Context, user, kind, product string, price int64, now time.Time) error {
	grants, err := s.offerCouponInputs(ctx, user, kind, product, now)
	if err != nil {
		return err
	}
	cohort := couponFingerprint(grants)
	h := sha256.Sum256([]byte(user + ":" + kind + ":" + product + ":" + strconv.FormatInt(price, 10) + ":" + cohort + ":" + now.UTC().Format("2006-01-02")))
	id := hex.EncodeToString(h[:])
	_, err = s.pricingCache.Load(ctx, "exposure:"+id, time.Hour, func() ([]byte, error) {
		var enabled int
		if e := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_product_settings WHERE product_type=? AND product_id=? AND dynamic_enabled=1)`, kind, product).Scan(&enabled); e != nil {
			return nil, e
		}
		if enabled == 0 {
			return nil, nil
		}
		reference, e := pricingReference(ctx, s.db, kind, product)
		if e != nil {
			return nil, e
		}
		observation, complete, watermark, e := pricingExposureInputs(ctx, s.db, user, now)
		if e != nil {
			return nil, e
		}
		discount, e := pricingCouponDiscount(grants, price)
		if e != nil {
			return nil, e
		}
		if len(grants) > 0 {
			observation.CouponPresent = 1
			observation.CouponDiscount = float64(discount) / float64(max(1, price))
		}
		observation.LogPrice = math.Log(float64(max(1, price)) / float64(max(1, reference)))
		m := pricing.ResponseModel{}
		model := "purchase-prior-v1"
		var raw string
		e = s.db.QueryRowContext(ctx, `SELECT id,parameters_json FROM pricing_model_versions WHERE kind=? AND frozen=0 AND created_at<=? ORDER BY created_at DESC LIMIT 1`, "response:"+kind+":"+product, pricingStamp(now)).Scan(&model, &raw)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return nil, e
		}
		if e == nil {
			if e = json.Unmarshal([]byte(raw), &m); e != nil {
				return nil, e
			}
		}
		feature, e := json.Marshal(map[string]any{"logPrice": observation.LogPrice, "activity": observation.Activity, "renewals": observation.Renewals, "couponFingerprint": cohort, "couponPresent": observation.CouponPresent, "couponDiscount": observation.CouponDiscount, "complete": complete, "watermark": watermark})
		if e != nil {
			return nil, e
		}
		_, e = s.db.ExecContext(ctx, `INSERT INTO pricing_offers(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, user, kind, product, price, reference, pricingStamp(now), pricingStamp(now.Add(24*time.Hour)), model, m.Predict(observation), string(feature))
		return nil, e
	})
	return err
}
