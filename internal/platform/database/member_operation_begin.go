package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

// MemberPurchaseFacts loads operation pricing and first-lineage state.
func (s *Store) MemberPurchaseFacts(ctx context.Context, purchaseID string) (purchaseops.PurchaseFacts, error) {
	purchase, err := s.PurchaseByID(ctx, purchaseID)
	if err != nil {
		return purchaseops.PurchaseFacts{}, err
	}
	var firstTerm int
	err = s.db.QueryRowContext(ctx, `SELECT CASE WHEN renewal_batch_id IS NULL AND auto_renew_source_purchase_id IS NULL
		AND NOT EXISTS(SELECT 1 FROM purchases successor WHERE successor.auto_renew_source_purchase_id=purchases.id)
		AND NOT EXISTS(SELECT 1 FROM renewal_batches WHERE source_purchase_id=purchases.id) THEN 1 ELSE 0 END
		FROM purchases WHERE id=?`, purchaseID).Scan(&firstTerm)
	return purchaseops.PurchaseFacts{Purchase: purchase, CoreGrossMinor: purchase.CoreGrossTXBMinor, FirstTerm: firstTerm == 1}, err
}

// ProviderOperationForActorKey checks a replay before mutable eligibility state.
func (s *Store) ProviderOperationForActorKey(ctx context.Context, actorID, kind, key, fingerprint string) (model.OperationReceipt, bool, error) {
	operation, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+
		` WHERE actor_user_id=? AND kind=? AND idempotency_key=?`, actorID, kind, strings.TrimSpace(key)))
	if errors.Is(err, ErrNotFound) {
		return model.OperationReceipt{}, false, nil
	}
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	if operation.RequestFingerprint != fingerprint {
		return model.OperationReceipt{}, true, ErrConflict
	}
	return operation.Receipt, true, nil
}

// BeginTrafficReset debits and creates a durable reset command atomically.
func (s *Store) BeginTrafficReset(ctx context.Context, input providerops.CreateInput, purchaseID string, now time.Time) (providerops.Operation, bool, error) {
	return s.beginMemberPurchaseOperation(ctx, input, purchaseID, true, now)
}

// BeginMemberRefund creates a durable refund command after an atomic local recheck.
func (s *Store) BeginMemberRefund(ctx context.Context, input providerops.CreateInput, purchaseID string, now time.Time) (providerops.Operation, bool, error) {
	return s.beginMemberPurchaseOperation(ctx, input, purchaseID, false, now)
}

func (s *Store) beginMemberPurchaseOperation(ctx context.Context, input providerops.CreateInput, purchaseID string, reset bool, now time.Time) (providerops.Operation, bool, error) {
	input, err := providerops.NormalizeCreate(input)
	if err != nil || len(input.Items) != 1 || input.Items[0].TargetID != purchaseID {
		return providerops.Operation{}, false, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, found, replayErr := memberOperationReplayTx(ctx, tx, input, now); found || replayErr != nil {
		if replayErr == nil {
			replayErr = tx.Commit()
		}
		return existing, found, replayErr
	}
	facts, err := memberPurchaseFactsTx(ctx, tx, purchaseID, input.OwnerUserID)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if !memberOperationEligible(facts, reset, now) {
		return providerops.Operation{}, false, purchaseops.ErrIneligible
	}
	if open, err := openMemberOperationTx(ctx, tx, purchaseID, input.Kind); err != nil || open {
		if err != nil {
			return providerops.Operation{}, false, err
		}
		return providerops.Operation{}, false, ErrConflict
	}
	operationID, err := insertMemberOperationTx(ctx, tx, input, now)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if reset {
		price, valid := purchaseops.ResetPriceMinor(facts.CoreGrossMinor, facts.Purchase.ResetStrategy)
		if !valid {
			return providerops.Operation{}, false, purchaseops.ErrIneligible
		}
		balance, err := changeBalanceTx(ctx, tx, input.OwnerUserID, -price, now)
		if err != nil {
			return providerops.Operation{}, false, err
		}
		if _, err := insertLedgerTx(ctx, tx, input.OwnerUserID, -price, balance, "traffic_reset_debit", operationID, "paid traffic reset", now); err != nil {
			return providerops.Operation{}, false, err
		}
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, operationID))
	if err == nil {
		err = tx.Commit()
	}
	return operation, false, err
}

func insertMemberOperationTx(ctx context.Context, tx *sql.Tx, input providerops.CreateInput, now time.Time) (string, error) {
	operationID, err := ids.New()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO provider_operations(id,actor_user_id,owner_user_id,kind,idempotency_key,
		request_fingerprint,status,created_at,updated_at) VALUES(?,?,?,?,?,?,'queued',?,?)`, operationID,
		input.ActorUserID, input.OwnerUserID, input.Kind, input.IdempotencyKey, input.RequestFingerprint, stamp(now), stamp(now))
	if err != nil {
		return "", err
	}
	item := input.Items[0]
	if _, err = tx.ExecContext(ctx, `INSERT INTO provider_operation_items(operation_id,item_key,target_type,target_id,status,created_at,updated_at)
		VALUES(?,?,?,?,'queued',?,?)`, operationID, item.Key, item.TargetType, item.TargetID, stamp(now), stamp(now)); err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{"operationId": operationID})
	if err != nil {
		return "", err
	}
	return operationID, insertOutboxTx(ctx, tx, providerops.OutboxKind, string(payload), now, now)
}
