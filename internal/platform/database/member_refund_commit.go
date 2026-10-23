package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

// FinalizeMemberRefund credits the original net debit and activates an independent successor atomically.
func (s *Store) FinalizeMemberRefund(ctx context.Context, operationID, purchaseID string, now time.Time) (purchaseops.RefundResult, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, operationID))
	if err != nil || operation.Receipt.Status != "processing" || operation.OwnerUserID == "" {
		return purchaseops.RefundResult{}, ErrConflict
	}
	facts, err := memberPurchaseFactsTx(ctx, tx, purchaseID, operation.OwnerUserID)
	if err != nil || !memberOperationEligible(facts, false, now) {
		return purchaseops.RefundResult{}, purchaseops.ErrIneligible
	}
	balance, err := changeBalanceTx(ctx, tx, operation.OwnerUserID, facts.Purchase.PriceTXBMinor, now)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if _, err = insertLedgerTx(ctx, tx, operation.OwnerUserID, facts.Purchase.PriceTXBMinor, balance,
		"member_refund_credit", operationID, "first-term purchase refund", now); err != nil {
		return purchaseops.RefundResult{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',auto_renew_enabled=0,updated_at=?
		WHERE id=? AND user_id=? AND status='active'`, stamp(now), purchaseID, operation.OwnerUserID)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return purchaseops.RefundResult{}, purchaseops.ErrIneligible
	}
	successorID, err := activateRefundSuccessorTx(ctx, tx, operation.OwnerUserID, now)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	resultJSON, err := json.Marshal(map[string]string{"purchaseId": purchaseID, "successorPurchaseId": successorID})
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',result_json=?,completed_at=?,updated_at=?
		WHERE operation_id=? AND item_key='purchase' AND status='processing'`, string(resultJSON), stamp(now), stamp(now), operationID); err != nil {
		return purchaseops.RefundResult{}, err
	}
	result, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded',result_json=?,completed_at=?,updated_at=?
		WHERE id=? AND status='processing'`, string(resultJSON), stamp(now), stamp(now), operationID)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return purchaseops.RefundResult{}, ErrConflict
	}
	if err := s.insertMemberRefundCompletedNoticeTx(ctx, tx, operationID, operation.OwnerUserID,
		facts.Purchase.ComboName, successorID, facts.Purchase.PriceTXBMinor, balance, now); err != nil {
		return purchaseops.RefundResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return purchaseops.RefundResult{}, err
	}
	if successorID == "" {
		return purchaseops.RefundResult{}, nil
	}
	successor, err := s.PurchaseByID(ctx, successorID)
	return purchaseops.RefundResult{Successor: &successor}, err
}

func activateRefundSuccessorTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (string, error) {
	var id, fromRaw string
	err := tx.QueryRowContext(ctx, `SELECT id,valid_from FROM purchases WHERE user_id=? AND status='queued'
		AND auto_renew_source_purchase_id IS NULL ORDER BY valid_from,created_at LIMIT 1`, userID).Scan(&id, &fromRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	from, err := parseStamp(fromRaw)
	if err != nil {
		return "", err
	}
	if err := shiftRefundQueueTx(ctx, tx, userID, from, now.Sub(from), now); err != nil {
		return "", err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='activating',traffic_reset_phase='pending',updated_at=?
		WHERE id=? AND status='queued'`, stamp(now), id)
	if err != nil {
		return "", err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return "", ErrConflict
	}
	if err := reconcilePurchaseContinuityTx(ctx, tx, id, "activating", now, now); err != nil {
		return "", err
	}
	return id, insertOutboxTx(ctx, tx, "xnode_apply_entitlement", `{"purchaseId":"`+id+`"}`, now, now)
}

func shiftRefundQueueTx(ctx context.Context, tx *sql.Tx, userID string, firstFrom time.Time, shift time.Duration, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,valid_from,valid_until FROM purchases
  WHERE user_id=? AND status='queued' AND valid_from>=? ORDER BY valid_from,created_at,id`, userID, stamp(firstFrom))
	if err != nil {
		return err
	}
	type queuedTerm struct{ id, from, until string }
	terms := make([]queuedTerm, 0)
	for rows.Next() {
		var term queuedTerm
		if err := rows.Scan(&term.id, &term.from, &term.until); err != nil {
			_ = rows.Close()
			return err
		}
		terms = append(terms, term)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for index, term := range terms {
		from, err := parseStamp(term.from)
		if err != nil {
			return err
		}
		until, err := parseStamp(term.until)
		if err != nil || !until.After(from) {
			return ErrConflict
		}
		if _, err = tx.ExecContext(ctx, `UPDATE purchases SET valid_from=?,valid_until=?,updated_at=? WHERE id=? AND status='queued'`,
			stamp(from.Add(shift)), stamp(until.Add(shift)), stamp(now), term.id); err != nil {
			return err
		}
		if index > 0 {
			if err := reconcilePurchaseContinuityTx(ctx, tx, term.id, "queued", from.Add(shift), now); err != nil {
				return err
			}
		}
	}
	return nil
}
