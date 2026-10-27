package database

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// ObservePricingWorker retains only a source-completeness watermark and explicit
// monitoring gaps. Raw application and node CPU/memory values stay memory-only.
func (s *Store) retryAttempt_ObservePricingWorker(ctx context.Context, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous string
	if err = tx.QueryRowContext(ctx, `SELECT observed_at FROM pricing_worker_watermark WHERE singleton=1`).Scan(&previous); err != nil {
		return err
	}
	if previous != "" {
		before, e := parseStamp(previous)
		if e != nil {
			return e
		}
		if now.Sub(before) > 45*time.Second {
			id, e := ids.New()
			if e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO pricing_operational_incidents(id,scope_type,scope_id,state,started_at,ended_at,reason) VALUES(?,'global','monitoring','TELEMETRY_STALE',?,?,'pricing monitoring continuity unavailable')`, id, previous, pricingStamp(now)); e != nil {
				return e
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE pricing_worker_watermark SET observed_at=? WHERE singleton=1`, pricingStamp(now)); err != nil {
		return err
	}
	return tx.Commit()
}

// ClosePricingOffers assigns nonpurchase only after the actual exposure window.
func (s *Store) ClosePricingOffers(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE pricing_offers SET outcome=0,observed_at=? WHERE id IN (SELECT id FROM pricing_offers WHERE outcome IS NULL AND expires_at<=? ORDER BY expires_at LIMIT 256)`, pricingStamp(now), pricingStamp(now))
	return err
}

// PricingOptimizationBatch runs at most four opted-in products per pass.
func (s *Store) PricingOptimizationBatch(ctx context.Context, now time.Time, cursor string) (string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT product_type,product_id FROM pricing_product_settings WHERE dynamic_enabled=1 AND product_type<>'payg' AND product_type||':'||product_id>? AND NOT EXISTS(SELECT 1 FROM pricing_decisions d WHERE d.product_type=pricing_product_settings.product_type AND d.product_id=pricing_product_settings.product_id AND d.policy_version>=pricing_product_settings.version AND d.generated_at>=CASE WHEN pricing_product_settings.product_type='payg' THEN ? ELSE ? END) ORDER BY product_type,product_id LIMIT 4`, cursor, pricingStamp(now.UTC().Truncate(time.Minute)), pricingStamp(now.UTC().Truncate(24*time.Hour)))
	if err != nil {
		return cursor, err
	}
	type item struct{ kind, id string }
	items := []item{}
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.kind, &i.id); err != nil {
			rows.Close()
			if errors.Is(err, ErrPricingComputePending) {
				continue
			}
			return cursor, err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return cursor, err
	}
	for _, i := range items {
		cursor = i.kind + ":" + i.id
		if i.kind == "combo" {
			if err = s.TrainPricingRenewalResponse(ctx, i.id, now); err != nil {
				if errors.Is(err, ErrPricingComputePending) {
					continue
				}
				return cursor, err
			}
		}
		if err = s.PricingProductForecasts(ctx, i.kind, i.id, now); err != nil {
			if errors.Is(err, ErrPricingComputePending) {
				continue
			}
			return cursor, err
		}
		err = s.PricingOptimizeProduct(ctx, i.kind, i.id, now)
		if err != nil {
			if errors.Is(err, ErrPricingComputePending) {
				continue
			}
			return cursor, err
		}
	}
	if len(items) < 4 {
		return "", nil
	}
	last := items[len(items)-1]
	return last.kind + ":" + last.id, nil
}
