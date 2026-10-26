package database

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) prepareAddonPricing(ctx context.Context, input PurchaseAddonInput, now time.Time) error {
	var combo string
	if err := s.db.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE id=? AND user_id=?`, input.PurchaseID, input.UserID).Scan(&combo); err != nil {
		return err
	}
	for _, id := range uniqueSorted(input.AddonSquadIDs) {
		if _, err := s.PreparePersonalPricing(ctx, input.UserID, "addon", id, combo, []string{id}, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) QuotePurchaseAddons(ctx context.Context, input PurchaseAddonInput, now time.Time) (model.PurchaseAddonQuote, error) {
	if err := s.prepareAddonPricing(ctx, input, now); err != nil {
		return model.PurchaseAddonQuote{}, err
	}
	return s.quotePurchaseAddons(ctx, input, now)
}

func (s *Store) AddPurchaseAddons(ctx context.Context, input PurchaseAddonInput, now time.Time) (model.Purchase, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_addon_adjustments WHERE purchase_id=? AND idempotency_key=?)`, input.PurchaseID, input.IdempotencyKey).Scan(&exists); err != nil {
		return model.Purchase{}, err
	}
	if exists == 0 {
		if err := s.prepareAddonPricing(ctx, input, now); err != nil {
			return model.Purchase{}, err
		}
	}
	p, err := s.addPurchaseAddons(ctx, input, now)
	if errors.Is(err, ErrInsufficientBalance) {
		for _, id := range input.AddonSquadIDs {
			if _, e := s.db.ExecContext(ctx, `UPDATE pricing_offers SET outcome_reason='INSUFFICIENT_FUNDS' WHERE user_id=? AND product_type='addon' AND product_id=? AND outcome IS NULL AND expires_at>?`, input.UserID, id, pricingStamp(now)); e != nil {
				return p, errors.Join(err, e)
			}
		}
	}
	return p, err
}
