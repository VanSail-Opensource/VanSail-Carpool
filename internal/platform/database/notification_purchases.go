package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func (s *Store) insertExpirationNotificationTx(ctx context.Context, tx *sql.Tx, purchaseID, gateKey string, now time.Time) error {
	var userID, combo, expired string
	err := tx.QueryRowContext(ctx, `SELECT purchases.user_id,combos.name,purchases.valid_until
		FROM purchases JOIN combos ON combos.id=purchases.combo_id WHERE purchases.id=?
		AND purchases.auto_renew_enabled=0 AND purchases.auto_renew_failure_reason='' AND NOT EXISTS (
			SELECT 1 FROM purchases successor WHERE successor.user_id=purchases.user_id AND successor.id<>purchases.id
			AND successor.status IN ('queued','activating','active') AND successor.valid_until>purchases.valid_until
		)`, purchaseID).Scan(&userID, &combo, &expired)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.insertUserNotificationTx(ctx, tx, "expired:"+purchaseID, userID, jobpayload.UserEventExpiration,
		gateKey, map[string]string{notifications.FactCombo: combo, notifications.FactExpired: expired}, now)
	return err
}

func (s *Store) insertActivationNotificationTx(ctx context.Context, tx *sql.Tx, purchaseID string, now time.Time) error {
	var userID, combo, reset, validUntil, validFrom, created string
	var traffic, debit int64
	var source sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT purchases.user_id,combos.name,
		COALESCE(purchases.entitlement_traffic_limit_bytes,combos.traffic_limit_bytes),
		COALESCE(purchases.entitlement_reset_strategy,combos.reset_strategy),purchases.valid_until,
		purchases.charged_txb_minor,purchases.auto_renew_source_purchase_id,purchases.valid_from,purchases.created_at
		FROM purchases JOIN combos ON combos.id=purchases.combo_id WHERE purchases.id=?`, purchaseID).Scan(
		&userID, &combo, &traffic, &reset, &validUntil, &debit, &source, &validFrom, &created)
	if err != nil {
		return err
	}
	if source.Valid {
		return s.insertAutoRenewalNotificationTx(ctx, tx, purchaseID, source.String, userID, combo, validUntil, debit, now)
	}
	starts, err := parseStamp(validFrom)
	if err != nil {
		return err
	}
	createdAt, err := parseStamp(created)
	if err != nil {
		return err
	}
	kind := jobpayload.UserEventPurchaseActivation
	if starts.After(createdAt) {
		kind = jobpayload.UserEventQueuedActivation
	}
	facts := map[string]string{
		notifications.FactCombo: combo, notifications.FactTrafficLimit: strconv.FormatInt(traffic, 10),
		notifications.FactReset: reset, notifications.FactValidUntil: validUntil,
	}
	if kind == jobpayload.UserEventPurchaseActivation {
		facts[notifications.FactCharge] = strconv.FormatInt(debit, 10)
	}
	if addOns, err := purchaseAddonSummaryTx(ctx, tx, purchaseID); err != nil {
		return err
	} else if addOns != "" {
		facts[notifications.FactAddOns] = addOns
	}
	_, err = s.insertUserNotificationTx(ctx, tx, "activation:"+purchaseID, userID,
		kind, "", facts, now)
	return err
}

func (s *Store) insertAutoRenewalNotificationTx(ctx context.Context, tx *sql.Tx, purchaseID, sourceID, userID, combo, validUntil string, debit int64, now time.Time) error {
	var balance int64
	if err := tx.QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id=?`, userID).Scan(&balance); err != nil {
		return err
	}
	facts := map[string]string{notifications.FactCombo: combo, notifications.FactRenewalDebit: strconv.FormatInt(-debit, 10), notifications.FactBalance: strconv.FormatInt(balance, 10), notifications.FactValidUntil: validUntil}
	_, err := s.insertUserNotificationTx(ctx, tx, "auto-renewal:"+purchaseID, userID, jobpayload.UserEventAutoRenewal, "", facts, now)
	return err
}

func purchaseAddonSummaryTx(ctx context.Context, tx *sql.Tx, purchaseID string) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT group_id FROM purchase_addons WHERE purchase_id=? ORDER BY group_id`, purchaseID)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return "", err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil || len(values) == 0 {
		return "", err
	}
	return squadSummary(values), nil
}
