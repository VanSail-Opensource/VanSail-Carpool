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
func (s *Store) retryAttempt_FinalizeMemberRefund(ctx context.Context, operationID, purchaseID string, now time.Time) (purchaseops.RefundResult, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1 FOR UPDATE`, operationID))
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if operation.Receipt.Kind != purchaseops.OperationRefundKind || operation.Receipt.Status != "processing" ||
		operation.OwnerUserID == "" || operation.ActorUserID != operation.OwnerUserID {
		return purchaseops.RefundResult{}, ErrConflict
	}
	var exactAttempt bool
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*)=1 AND COUNT(*) FILTER (
		WHERE item_key='purchase' AND target_type='purchase' AND target_id=$2
		AND status='processing' AND attempt_started_at IS NOT NULL)=1
		FROM provider_operation_items WHERE operation_id=$1`, operationID, purchaseID).Scan(&exactAttempt); err != nil {
		return purchaseops.RefundResult{}, err
	}
	if !exactAttempt {
		return purchaseops.RefundResult{}, ErrConflict
	}
	facts, err := memberPurchaseFactsTx(ctx, tx, purchaseID, operation.OwnerUserID)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if !memberOperationEligible(facts, now) {
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
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',auto_renew_enabled=0,updated_at=$1
		WHERE id=$2 AND user_id=$3 AND status='active'`, stamp(now), purchaseID, operation.OwnerUserID)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return purchaseops.RefundResult{}, rowsErr
	} else if affected != 1 {
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
	result, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',result_json=$1,completed_at=$2,updated_at=$3
		WHERE operation_id=$4 AND item_key='purchase' AND target_type='purchase' AND target_id=$5
		AND status='processing' AND attempt_started_at IS NOT NULL`, string(resultJSON), stamp(now), stamp(now), operationID, purchaseID)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return purchaseops.RefundResult{}, rowsErr
	} else if affected != 1 {
		return purchaseops.RefundResult{}, ErrConflict
	}
	result, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded',result_json=$1,completed_at=$2,updated_at=$3
		WHERE id=$4 AND status='processing'`, string(resultJSON), stamp(now), stamp(now), operationID)
	if err != nil {
		return purchaseops.RefundResult{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return purchaseops.RefundResult{}, rowsErr
	} else if affected != 1 {
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
	err := tx.QueryRowContext(ctx, `SELECT id,valid_from FROM purchases WHERE user_id=$1 AND status='queued'
		AND auto_renew_source_purchase_id IS NULL ORDER BY vansail_timestamp(valid_from),created_at,id LIMIT 1`, userID).Scan(&id, &fromRaw)
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
	if err := shiftRefundQueueTx(ctx, tx, userID, from, now, now); err != nil {
		return "", err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='activating',traffic_reset_phase='pending',updated_at=$1
		WHERE id=$2 AND status='queued'`, stamp(now), id)
	if err != nil {
		return "", err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return "", ErrConflict
	}
	if err := retirePurchaseContinuityTx(ctx, tx, id, now); err != nil {
		return "", err
	}
	return id, insertOutboxTx(ctx, tx, "xnode_apply_entitlement", `{"purchaseId":"`+id+`"}`, now, now)
}
