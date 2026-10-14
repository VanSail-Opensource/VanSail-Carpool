package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func operationComboTx(ctx context.Context, tx *sql.Tx, operationID string) (string, error) {
	var combo string
	err := tx.QueryRowContext(ctx, `SELECT combos.name FROM provider_operation_items item
		JOIN purchases ON purchases.id=item.target_id JOIN combos ON combos.id=purchases.combo_id
		WHERE item.operation_id=? AND item.item_key='purchase'`, operationID).Scan(&combo)
	return combo, err
}

func operationLedgerTx(ctx context.Context, tx *sql.Tx, operationID, kind string) (int64, int64, error) {
	var delta, balance int64
	err := tx.QueryRowContext(ctx, `SELECT delta_txb_minor,balance_after FROM ledger_entries
		WHERE kind=? AND reference_id=?`, kind, operationID).Scan(&delta, &balance)
	return delta, balance, err
}

func (s *Store) insertManualResetNoticeTx(ctx context.Context, tx *sql.Tx, operation providerops.Operation,
	refunded bool, reason string, now time.Time) error {
	if purchaseops.IsAutomaticTrafficResetKey(operation.IdempotencyKey) {
		return nil
	}
	combo, err := operationComboTx(ctx, tx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	ledgerKind, eventKind, key := "traffic_reset_debit", jobpayload.UserEventManualResetCompleted, "manual-reset-completed:"
	if refunded {
		ledgerKind, eventKind, key = "traffic_reset_compensation", jobpayload.UserEventManualResetRefunded, "manual-reset-refunded:"
	}
	amount, balance, err := operationLedgerTx(ctx, tx, operation.Receipt.ID, ledgerKind)
	if err != nil {
		return err
	}
	if amount < 0 {
		amount = -amount
	}
	facts := map[string]string{
		notifications.FactCombo: combo, notifications.FactBalance: strconv.FormatInt(balance, 10),
		notifications.FactTime: stamp(now),
	}
	if refunded {
		if reason == "" {
			reason = "RESET_FAILED"
		}
		facts[notifications.FactAmount], facts[notifications.FactReason] = strconv.FormatInt(amount, 10), reason
	} else {
		facts[notifications.FactCharge] = strconv.FormatInt(amount, 10)
	}
	_, err = s.insertUserNotificationTx(ctx, tx, key+operation.Receipt.ID, operation.OwnerUserID, eventKind, "", facts, now)
	return err
}

func (s *Store) insertMemberRefundFailedNoticeTx(ctx context.Context, tx *sql.Tx, operation providerops.Operation,
	reason string, now time.Time) error {
	if reason == "" {
		reason = "REFUND_UNAVAILABLE"
	}
	combo, err := operationComboTx(ctx, tx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	_, err = s.insertUserNotificationTx(ctx, tx, "member-refund-failed:"+operation.Receipt.ID, operation.OwnerUserID,
		jobpayload.UserEventMemberRefundFailed, "", map[string]string{
			notifications.FactCombo: combo, notifications.FactReason: reason,
			notifications.FactTime: stamp(now),
		}, now)
	return err
}

func (s *Store) insertMemberRefundCompletedNoticeTx(ctx context.Context, tx *sql.Tx, operationID, userID, combo,
	replacementID string, amount, balance int64, now time.Time) error {
	gate := ""
	facts := map[string]string{
		notifications.FactCombo: combo, notifications.FactAmount: strconv.FormatInt(amount, 10),
		notifications.FactBalance: strconv.FormatInt(balance, 10), notifications.FactTime: stamp(now),
	}
	if replacementID != "" {
		var replacement string
		if err := tx.QueryRowContext(ctx, `SELECT combos.name FROM purchases JOIN combos ON combos.id=purchases.combo_id
			WHERE purchases.id=?`, replacementID).Scan(&replacement); err != nil {
			return err
		}
		facts[notifications.FactReplacement] = replacement
		gate = userSyncGate(userID)
	}
	_, err := s.insertUserNotificationTx(ctx, tx, "member-refund-completed:"+operationID, userID,
		jobpayload.UserEventMemberRefundCompleted, gate, facts, now)
	return err
}
