package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PreparePurchasePricing gathers numerical work before commerce takes a write
// lock. Pending work never changes a renewal preference or reserves a coupon.
func (s *Store) retryAttempt_PreparePurchasePricing(ctx context.Context, input PurchaseInput, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	c, err := comboByIDTx(ctx, tx, input.ComboID, true)
	if err != nil {
		tx.Rollback()
		return err
	}
	squads := append([]string{}, input.AddonSquadIDs...)
	for _, g := range c.IncludedSquads {
		squads = append(squads, g.GroupID)
	}
	var current string
	err = tx.QueryRowContext(ctx, `SELECT id FROM purchases WHERE user_id=? AND status IN ('active','activating') AND valid_until>? ORDER BY valid_until DESC,id LIMIT 1`, input.UserID, pricingStamp(now)).Scan(&current)
	tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err = s.PreparePersonalPricing(ctx, input.UserID, "combo", c.ID, c.ID, squads, now); err != nil {
		return err
	}
	for _, id := range uniqueSorted(input.AddonSquadIDs) {
		if _, err = s.PreparePersonalPricing(ctx, input.UserID, "addon", id, c.ID, []string{id}, now); err != nil {
			return err
		}
	}
	if current != "" {
		err = s.prepareOwnedPricingRenewal(ctx, input.UserID, current, now)
	}
	return err
}

func (s *Store) CreatePurchase(ctx context.Context, input PurchaseInput, now time.Time) (model.Purchase, error) {
	// A replay must remain available after its original decision expires.
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND idempotency_key=?)`, input.UserID, input.IdempotencyKey).Scan(&exists)
	if err != nil {
		return model.Purchase{}, err
	}
	if exists == 0 {
		if err = s.PreparePurchasePricing(ctx, input, now); err != nil {
			return model.Purchase{}, err
		}
	}
	p, err := s.createPurchase(ctx, input, now)
	if errors.Is(err, ErrInsufficientBalance) {
		_, e := s.db.ExecContext(ctx, `UPDATE pricing_offers SET outcome_reason='INSUFFICIENT_FUNDS' WHERE user_id=? AND product_type='combo' AND product_id=? AND outcome IS NULL AND expires_at>?`, input.UserID, input.ComboID, pricingStamp(now))
		if e != nil {
			return p, errors.Join(err, e)
		}
	}
	return p, err
}

func (s *Store) queuedRenewalDecisionTx(ctx context.Context, tx *sql.Tx, user string, now time.Time) (*model.PricingDecision, error) {
	var current, product string
	err := tx.QueryRowContext(ctx, `SELECT id,combo_id FROM purchases WHERE user_id=? AND status IN ('active','activating') AND valid_until>? ORDER BY valid_until DESC,id LIMIT 1`, user, pricingStamp(now)).Scan(&current, &product)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := loadPricingPolicy(ctx, tx, "combo", product, 0)
	if err != nil {
		return nil, err
	}
	if !p.DynamicEnabled || p.RenewalMode != "DYNAMIC" {
		return fixedRenewalDecisionTx(ctx, tx, user, current, p.Version, now)
	}
	c, err := pricingContractTx(ctx, tx, current)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPricingDecisionPending
	}
	if err != nil {
		return nil, err
	}
	q, err := scanPricingQuote(tx.QueryRowContext(ctx, pricingQuoteSelect+` WHERE contract_id=? AND contract_version=? AND state IN ('AUTHORIZED','READY_TO_SETTLE','DEFERRED_HEALTH') AND valid_until>? ORDER BY generated_at DESC LIMIT 1`, c.ID, c.Version, pricingStamp(now)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPricingDecisionPending
	}
	if err != nil {
		return nil, err
	}
	p, err = loadPricingPolicy(ctx, tx, "combo", product, c.Price)
	if err != nil {
		return nil, err
	}
	if p.Version != q.PolicyVersion {
		return nil, ErrPricingDecisionPending
	}
	in, err := s.renewalPriceSnapshotTx(ctx, tx, c, now)
	if err != nil {
		return nil, err
	}
	var evidence struct {
		Hash        string `json:"snapshotHash"`
		Reason      string `json:"reason"`
		Calibration string `json:"calibrationStatus"`
	}
	if json.Unmarshal([]byte(q.Trace), &evidence) != nil || evidence.Hash != in.Hash {
		return nil, ErrPricingDecisionPending
	}
	net, err := renewalNet(in, q.Price)
	if err != nil {
		return nil, err
	}
	status := "UNCHANGED"
	if q.Price != c.Price {
		status = "ADJUSTED"
	}
	return &model.PricingDecision{DynamicEnabled: true, Calibration: evidence.Calibration, Version: q.ID, Status: status, Reason: evidence.Reason, PersonalPrice: model.TXBMoney(q.Price), PublicPrice: model.TXBMoney(in.Public), NetPrice: model.TXBMoney(net), ModelVersion: q.ModelVersion, GeneratedAt: q.GeneratedAt, ValidUntil: q.ValidUntil, Probability: &q.Probability}, nil
}
