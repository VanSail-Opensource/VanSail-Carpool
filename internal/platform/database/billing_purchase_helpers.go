package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strings"
	"time"
)

func checkSquadStockTx(ctx context.Context, tx *sql.Tx, squadUUIDs []string, holderUserID ...string) error {
	for _, squadUUID := range squadUUIDs {
		var limit sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT stock_limit FROM squad_product_overrides WHERE group_id=?`, squadUUID).Scan(&limit); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return fmt.Errorf("load squad stock: %w", err)
		}
		if !limit.Valid {
			continue
		}
		var reservations, heldByUser int64
		holderID := ""
		if len(holderUserID) > 0 {
			holderID = holderUserID[0]
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id),COALESCE(MAX(CASE WHEN user_id=? THEN 1 ELSE 0 END),0) FROM purchases
			WHERE status IN ('activating','active','queued') AND (EXISTS (
				SELECT 1 FROM json_each((SELECT included_squad_uuids FROM combos WHERE combos.id=purchases.combo_id)) WHERE value=?
			) OR EXISTS (SELECT 1 FROM purchase_addons WHERE purchase_addons.purchase_id=purchases.id AND group_id=?))`, holderID, squadUUID, squadUUID).Scan(&reservations, &heldByUser); err != nil {
			return fmt.Errorf("count squad reservations: %w", err)
		}
		// Capacity counts distinct members. Continuing an existing reservation
		// needs no new seat, even if the limit was lowered or sales were closed.
		if heldByUser == 1 {
			continue
		}
		if reservations >= limit.Int64 {
			return ErrStockUnavailable
		}
	}
	return nil
}

func nextPurchaseStartTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (time.Time, error) {
	validFrom := now
	var latestEnd string
	err := tx.QueryRowContext(ctx, `SELECT valid_until FROM purchases WHERE user_id=? AND status IN ('activating','active','queued') ORDER BY valid_until DESC LIMIT 1`, userID).Scan(&latestEnd)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, fmt.Errorf("find current entitlement: %w", err)
	}
	if err == nil {
		parsed, parseErr := parseStamp(latestEnd)
		if parseErr != nil {
			return time.Time{}, fmt.Errorf("parse current entitlement: %w", parseErr)
		}
		if parsed.After(validFrom) {
			validFrom = parsed
		}
	}
	return validFrom, nil
}

func normalizeAndFingerprintPurchase(input *PurchaseInput) (string, error) {
	if input == nil {
		return "", errors.New("purchase input is required")
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.ComboID = strings.TrimSpace(input.ComboID)
	input.CouponGrantID = strings.TrimSpace(input.CouponGrantID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.UserID == "" || input.ComboID == "" || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		return "", errors.New("purchase user, combo, and idempotency key are required")
	}
	normalizedAddons := make([]string, len(input.AddonSquadIDs))
	for index := range input.AddonSquadIDs {
		normalizedAddons[index] = strings.TrimSpace(input.AddonSquadIDs[index])
	}
	input.AddonSquadIDs = uniqueSorted(normalizedAddons)
	activationIDs := make([]string, 0, len(input.SquadActivationCodes))
	for uuid := range input.SquadActivationCodes {
		activationIDs = append(activationIDs, strings.TrimSpace(uuid))
	}
	activationIDs = uniqueSorted(activationIDs)
	payload, err := json.Marshal(struct {
		Version       int      `json:"version"`
		ComboID       string   `json:"comboId"`
		AddonSquadIDs []string `json:"addonSquadIds"`
		CouponGrantID string   `json:"couponGrantId"`
		ActivationIDs []string `json:"activationIds"`
	}{Version: 2, ComboID: input.ComboID, AddonSquadIDs: input.AddonSquadIDs, CouponGrantID: input.CouponGrantID, ActivationIDs: activationIDs})
	if err != nil {
		return "", fmt.Errorf("encode purchase request fingerprint: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func comboByIDTx(ctx context.Context, tx *sql.Tx, id string, activeOnly bool) (model.Combo, error) {
	query := comboSelect + ` WHERE id=?`
	if activeOnly {
		query += ` AND active=1`
	}
	return scanCombo(tx.QueryRowContext(ctx, query, id))
}

func squadByIDTx(ctx context.Context, tx *sql.Tx, id string, requireVisible bool) (model.SquadProduct, error) {
	query := squadSelect + ` WHERE group_id=?`
	if requireVisible {
		query += ` AND visible=1`
	}
	return scanSquad(tx.QueryRowContext(ctx, query, id))
}

func debitBalanceTx(ctx context.Context, tx *sql.Tx, userID string, amount int64, now time.Time) (int64, error) {
	if amount < 0 {
		return 0, errors.New("negative debit")
	}
	var balance int64
	err := tx.QueryRowContext(ctx, `UPDATE balances SET txb_minor=txb_minor-?,updated_at=? WHERE user_id=? AND txb_minor>=? RETURNING txb_minor`, amount, stamp(now), userID, amount).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInsufficientBalance
	}
	if err != nil {
		return 0, fmt.Errorf("debit balance: %w", err)
	}
	return balance, nil
}
