package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

// CompensateTrafficReset credits one failed reset debit and closes its receipt atomically.
func (s *Store) CompensateTrafficReset(ctx context.Context, operationID, errorCode string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, operationID))
	if err != nil {
		return err
	}
	if operation.Receipt.Status == string(providerops.StatusCompensated) {
		return nil
	}
	if operation.Receipt.Status != string(providerops.StatusProcessing) {
		return ErrConflict
	}
	var userID string
	var debit int64
	err = tx.QueryRowContext(ctx, `SELECT user_id,delta_txb_minor FROM ledger_entries
		WHERE kind='traffic_reset_debit' AND reference_id=?`, operationID).Scan(&userID, &debit)
	if err != nil {
		return fmt.Errorf("load traffic reset debit: %w", err)
	}
	if debit >= 0 {
		return errors.New("traffic reset debit is invalid")
	}
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id FROM ledger_entries WHERE kind='traffic_reset_compensation' AND reference_id=?`, operationID).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		balance, changeErr := changeBalanceTx(ctx, tx, userID, -debit, now)
		if changeErr != nil {
			return changeErr
		}
		if _, changeErr = insertLedgerTx(ctx, tx, userID, -debit, balance, "traffic_reset_compensation", operationID, "traffic reset compensation", now); changeErr != nil {
			return changeErr
		}
	}
	code := operationCode(errorCode)
	if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='compensated',error_code=?,result_json='{}',
		completed_at=?,updated_at=? WHERE operation_id=? AND status='processing'`, code, stamp(now), stamp(now), operationID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status='compensated',error_code=?,result_json='{}',
		completed_at=?,updated_at=? WHERE id=? AND status='processing'`, code, stamp(now), stamp(now), operationID)
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return ErrConflict
	}
	if purchaseops.IsAutomaticTrafficResetKey(operation.IdempotencyKey) {
		var combo string
		if err := tx.QueryRowContext(ctx, `SELECT combos.name FROM provider_operation_items item
			JOIN purchases ON purchases.id=item.target_id JOIN combos ON combos.id=purchases.combo_id
			WHERE item.operation_id=? AND item.item_key='purchase'`, operationID).Scan(&combo); err != nil {
			return err
		}
		balance, err := balanceTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		if _, err := s.insertUserNotificationTx(ctx, tx, "automatic-reset-failed:"+operationID, userID,
			jobpayload.UserEventAutomaticResetFailed, "", map[string]string{
				notifications.FactCombo: combo, notifications.FactAmount: strconv.FormatInt(-debit, 10),
				notifications.FactBalance: strconv.FormatInt(balance, 10), notifications.FactReason: code,
				notifications.FactTime: now.UTC().Format(time.RFC3339Nano),
			}, now); err != nil {
			return err
		}
	} else if err := s.insertManualResetNoticeTx(ctx, tx, operation, true, code, now); err != nil {
		return err
	}
	return tx.Commit()
}
