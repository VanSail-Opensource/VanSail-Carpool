package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strings"
	"time"
)

// RenewalInput describes one contiguous renewal batch.
type RenewalInput struct {
	UserID, PurchaseID, IdempotencyKey string
	TermCount                          int
}

func (s *Store) retryAttempt_RenewalQuote(ctx context.Context, userID, purchaseID string, termCount int, now time.Time) (model.RenewalQuote, error) {
	if termCount < 1 || termCount > 6 {
		return model.RenewalQuote{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.RenewalQuote{}, err
	}
	defer func() { _ = tx.Rollback() }()
	quote, _, _, err := renewalQuoteTx(ctx, tx, userID, purchaseID, termCount, nil, now.UTC())
	return quote, err
}

// RenewalQuoteExcludingAddons previews a renewal without live-unavailable paid squads.
func (s *Store) retryAttempt_RenewalQuoteExcludingAddons(ctx context.Context, userID, purchaseID string, termCount int, excludedAddonIDs []string, now time.Time) (model.RenewalQuote, error) {
	if termCount < 1 || termCount > 6 {
		return model.RenewalQuote{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.RenewalQuote{}, err
	}
	defer func() { _ = tx.Rollback() }()
	quote, _, _, err := renewalQuoteTx(ctx, tx, userID, purchaseID, termCount, excludedAddonIDs, now.UTC())
	return quote, err
}

// Renew creates all terms and one ledger debit in one SQLite transaction.
func (s *Store) Renew(ctx context.Context, input RenewalInput, now time.Time) (model.RenewalBatch, error) {
	return s.renew(ctx, input, nil, now)
}

// RenewExcludingAddons creates a renewal without live-unavailable paid squads.
func (s *Store) RenewExcludingAddons(ctx context.Context, input RenewalInput, excludedAddonIDs []string, now time.Time) (model.RenewalBatch, error) {
	return s.renew(ctx, input, excludedAddonIDs, now)
}

func (s *Store) retryAttempt_renew(ctx context.Context, input RenewalInput, excludedAddonIDs []string, now time.Time) (model.RenewalBatch, error) {
	if input.TermCount < 1 || input.TermCount > 6 || strings.TrimSpace(input.IdempotencyKey) == "" {
		return model.RenewalBatch{}, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.RenewalBatch{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var existingID, existingFingerprint string
	fingerprint := fmt.Sprintf("%s:%d", input.PurchaseID, input.TermCount)
	err = tx.QueryRowContext(ctx, `SELECT id,request_fingerprint FROM renewal_batches WHERE user_id=? AND idempotency_key=?`, input.UserID, input.IdempotencyKey).Scan(&existingID, &existingFingerprint)
	if err == nil {
		if existingID == "" || existingFingerprint != fingerprint {
			return model.RenewalBatch{}, ErrConflict
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return model.RenewalBatch{}, rollbackErr
		}
		return s.loadRenewalBatch(ctx, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.RenewalBatch{}, err
	}
	quote, combo, addons, err := renewalQuoteTx(ctx, tx, input.UserID, input.PurchaseID, input.TermCount, excludedAddonIDs, now.UTC())
	if err != nil {
		return model.RenewalBatch{}, err
	}
	var traffic sql.NullInt64
	var reset, squads sql.NullString
	var customPrice, rollover, renewalTraffic sql.NullInt64
	var trafficRenewal int
	if err = tx.QueryRowContext(ctx, `SELECT entitlement_traffic_limit_bytes,entitlement_reset_strategy,entitlement_squad_uuids,
		reward_renewal_price_minor,reward_rollover_min_remaining_bps,reward_traffic_renewal,
		reward_renewal_traffic_limit_bytes FROM purchases WHERE id=?`,
		input.PurchaseID).Scan(&traffic, &reset, &squads, &customPrice, &rollover, &trafficRenewal, &renewalTraffic); err != nil {
		return model.RenewalBatch{}, err
	}
	if trafficRenewal == 0 {
		traffic = renewalTraffic
	}
	if squads.Valid {
		squads.String = squadJSON(combo.IncludedSquads)
	}
	batchID, err := ids.New()
	if err != nil {
		return model.RenewalBatch{}, err
	}
	newBalance, err := debitBalanceTx(ctx, tx, input.UserID, quote.TotalPrice.MinorInt64(), now.UTC())
	if err != nil {
		return model.RenewalBatch{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO renewal_batches(id,user_id,source_purchase_id,idempotency_key,request_fingerprint,term_count,charged_txb_minor,created_at) VALUES(?,?,?,?,?,?,?,?)`, batchID, input.UserID, input.PurchaseID, input.IdempotencyKey, fingerprint, input.TermCount, quote.TotalPrice.MinorInt64(), stamp(now)); err != nil {
		return model.RenewalBatch{}, err
	}
	start := quote.EffectiveAt
	idsForBatch := make([]string, 0, input.TermCount)
	for index := 0; index < input.TermCount; index++ {
		purchaseID, err := ids.New()
		if err != nil {
			return model.RenewalBatch{}, err
		}
		from := start.AddDate(0, 0, combo.ValidityDays*index)
		until := from.AddDate(0, 0, combo.ValidityDays)
		status := "queued"
		if !from.After(now) {
			status = "activating"
		}
		requestKey := any(nil)
		if index == 0 {
			requestKey = input.IdempotencyKey
		}
		coreGross := combo.PriceTXBMinor
		if customPrice.Valid {
			coreGross = customPrice.Int64
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,coupon_grant_id,gross_price_txb_minor,core_gross_txb_minor,coupon_discount_txb_minor,recurring_discount_attached,idempotency_key,request_fingerprint,renewal_batch_id,renewal_index,entitlement_traffic_limit_bytes,entitlement_reset_strategy,entitlement_squad_uuids,reward_renewal_price_minor,reward_rollover_min_remaining_bps,reward_traffic_renewal,reward_renewal_traffic_limit_bytes,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, purchaseID, input.UserID, combo.ID, quote.PricePerTerm.MinorInt64(), stamp(from), stamp(until), status, quote.CouponGrantID, quote.GrossPrice.MinorInt64(), coreGross, quote.Discount.MinorInt64(), boolInt(quote.CouponGrantID != nil), requestKey, fingerprint, batchID, index,
			traffic, reset, squads, customPrice, rollover, 1, traffic, stamp(now), stamp(now)); err != nil {
			return model.RenewalBatch{}, err
		}
		if err := snapshotComboSquadsTx(ctx, tx, purchaseID, combo.IncludedSquads); err != nil {
			return model.RenewalBatch{}, err
		}
		for _, addon := range addons {
			if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_addons(purchase_id,group_id,charged_txb_minor) VALUES(?,?,?)`, purchaseID, addon.GroupID, addon.PriceTXBMinor); err != nil {
				return model.RenewalBatch{}, err
			}
		}
		if err := enqueuePurchaseTransitionTx(ctx, tx, purchaseID, status, from, now); err != nil {
			return model.RenewalBatch{}, err
		}
		idsForBatch = append(idsForBatch, purchaseID)
	}
	if _, err := insertLedgerTx(ctx, tx, input.UserID, -quote.TotalPrice.MinorInt64(), newBalance, "purchase_debit", batchID, "renewal batch", now); err != nil {
		return model.RenewalBatch{}, err
	}
	if err := s.insertRenewalScheduledNoticeTx(ctx, tx, batchID, input.UserID, combo.Name, input.TermCount,
		quote.TotalPrice.MinorInt64(), newBalance, start, start.AddDate(0, 0, combo.ValidityDays*input.TermCount), now); err != nil {
		return model.RenewalBatch{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.RenewalBatch{}, err
	}
	return s.loadRenewalBatch(ctx, batchID)
}
