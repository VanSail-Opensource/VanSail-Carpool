package database

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// CancelAdminPurchase atomically cancels, credits, audits, and gates delivery.
func (s *Store) CancelAdminPurchase(ctx context.Context, actorID, purchaseID, reason string, now time.Time) (model.Purchase, error) {
	reason = strings.TrimSpace(reason)
	if actorID == "" || purchaseID == "" || reason == "" || len(reason) > 500 {
		return model.Purchase{}, errors.New("administrator cancellation is invalid")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, err
	}
	defer func() { _ = tx.Rollback() }()
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=?`, purchaseID))
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.Status == "cancelled" {
		return purchase, nil
	}
	if purchase.Status == "expired" || purchase.Status == "failed" {
		return model.Purchase{}, ErrConflict
	}
	previousStatus := purchase.Status
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=? WHERE id=?`, stamp(now), purchase.ID); err != nil {
		return model.Purchase{}, err
	}
	balance, err := changeBalanceTx(ctx, tx, purchase.UserID, purchase.PriceTXBMinor, now)
	if err != nil {
		return model.Purchase{}, err
	}
	ledgerID, err := insertLedgerTx(ctx, tx, purchase.UserID, purchase.PriceTXBMinor, balance,
		"admin_entitlement_cancellation", purchase.ID, reason, now)
	if err != nil {
		return model.Purchase{}, err
	}
	detail, err := json.Marshal(map[string]any{"reason": reason, "ledgerEntryId": ledgerID})
	if err != nil {
		return model.Purchase{}, err
	}
	auditID, err := ids.New()
	if err != nil {
		return model.Purchase{}, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, "entitlement.cancel", "purchase", purchase.ID, string(detail), now); err != nil {
		return model.Purchase{}, err
	}
	gate := ""
	if previousStatus != "queued" {
		gate = userSyncGate(purchase.UserID)
		if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+purchase.UserID+`"}`, now, now); err != nil {
			return model.Purchase{}, err
		}
	}
	facts := adminFinanceFacts("entitlement_cancel", 0, balance, reason, now)
	delete(facts, notifications.FactAmount)
	facts[notifications.FactCombo] = purchase.ComboName
	facts[notifications.FactCredited] = strconv.FormatInt(purchase.PriceTXBMinor, 10)
	facts[notifications.FactPreviousStatus], facts[notifications.FactNewStatus] = previousStatus, "cancelled"
	if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+ledgerID+":"+purchase.UserID,
		purchase.UserID, jobpayload.UserEventAdminUpdate, gate, facts, now); err != nil {
		return model.Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, err
	}
	return s.PurchaseByID(ctx, purchase.ID)
}
