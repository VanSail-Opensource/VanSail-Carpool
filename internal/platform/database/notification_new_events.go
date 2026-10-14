package database

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func productNames(products []model.SquadProduct) string {
	names := make([]string, 0, len(products))
	for _, product := range products {
		if name := strings.TrimSpace(product.Name); name != "" {
			names = append(names, name)
		} else {
			names = append(names, product.RemnaSquadUUID)
		}
	}
	return strings.Join(names, ", ")
}

func (s *Store) insertQueuedPurchaseNoticeTx(ctx context.Context, tx *sql.Tx, purchaseID, userID string,
	combo model.Combo, products []model.SquadProduct, charge, balance int64, from, until, now time.Time) error {
	facts := map[string]string{
		notifications.FactCombo: combo.Name, notifications.FactCharge: strconv.FormatInt(charge, 10),
		notifications.FactBalance:   strconv.FormatInt(balance, 10),
		notifications.FactValidFrom: stamp(from), notifications.FactValidUntil: stamp(until),
	}
	if len(products) > 0 {
		facts[notifications.FactAddOns] = productNames(products)
	}
	_, err := s.insertUserNotificationTx(ctx, tx, "purchase-queued:"+purchaseID, userID,
		jobpayload.UserEventPurchaseQueued, "", facts, now)
	return err
}

func (s *Store) insertRenewalScheduledNoticeTx(ctx context.Context, tx *sql.Tx, batchID, userID, combo string,
	terms int, charge, balance int64, from, until, now time.Time) error {
	_, err := s.insertUserNotificationTx(ctx, tx, "renewal-scheduled:"+batchID, userID,
		jobpayload.UserEventRenewalScheduled, "", map[string]string{
			notifications.FactCombo: combo, notifications.FactTermCount: strconv.Itoa(terms),
			notifications.FactCharge: strconv.FormatInt(charge, 10), notifications.FactBalance: strconv.FormatInt(balance, 10),
			notifications.FactValidFrom: stamp(from), notifications.FactValidUntil: stamp(until),
		}, now)
	return err
}

func (s *Store) insertAddonNoticeTx(ctx context.Context, tx *sql.Tx, adjustmentID, userID, purchaseID string,
	products []model.SquadProduct, charge, balance int64, now time.Time) error {
	var combo, until string
	if err := tx.QueryRowContext(ctx, `SELECT combos.name,purchases.valid_until FROM purchases
		JOIN combos ON combos.id=purchases.combo_id WHERE purchases.id=?`, purchaseID).Scan(&combo, &until); err != nil {
		return err
	}
	_, err := s.insertUserNotificationTx(ctx, tx, "addon-activated:"+adjustmentID, userID,
		jobpayload.UserEventAddonActivated, userSyncGate(userID), map[string]string{
			notifications.FactCombo: combo, notifications.FactAddOns: productNames(products),
			notifications.FactCharge: strconv.FormatInt(charge, 10), notifications.FactBalance: strconv.FormatInt(balance, 10),
			notifications.FactValidUntil: until, notifications.FactTime: stamp(now),
		}, now)
	return err
}

func (s *Store) insertQueuedCancellationNoticeTx(ctx context.Context, tx *sql.Tx, purchase model.Purchase,
	balance int64, now time.Time) error {
	_, err := s.insertUserNotificationTx(ctx, tx, "queued-cancellation:"+purchase.ID, purchase.UserID,
		jobpayload.UserEventQueuedCancellation, "", map[string]string{
			notifications.FactCombo:   purchase.ComboName,
			notifications.FactAmount:  strconv.FormatInt(purchase.PriceTXBMinor, 10),
			notifications.FactBalance: strconv.FormatInt(balance, 10), notifications.FactTime: stamp(now),
		}, now)
	return err
}

func (s *Store) insertAutoRenewalFailureNoticeTx(ctx context.Context, tx *sql.Tx, purchaseID, userID, reason, gate string,
	now time.Time) error {
	var combo, expired string
	if err := tx.QueryRowContext(ctx, `SELECT combos.name,purchases.valid_until FROM purchases
		JOIN combos ON combos.id=purchases.combo_id WHERE purchases.id=?`, purchaseID).Scan(&combo, &expired); err != nil {
		return err
	}
	_, err := s.insertUserNotificationTx(ctx, tx, "auto-renewal-failed:"+purchaseID, userID,
		jobpayload.UserEventAutoRenewalFailed, gate, map[string]string{
			notifications.FactCombo: combo, notifications.FactExpired: expired, notifications.FactReason: reason,
		}, now)
	return err
}
