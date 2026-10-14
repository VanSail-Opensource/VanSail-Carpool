package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func memberOperationReplayTx(ctx context.Context, tx *sql.Tx, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	existing, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+
		` WHERE actor_user_id=? AND kind=? AND idempotency_key=?`, input.ActorUserID, input.Kind, input.IdempotencyKey))
	if errors.Is(err, ErrNotFound) {
		return providerops.Operation{}, false, nil
	}
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if existing.RequestFingerprint != input.RequestFingerprint {
		return providerops.Operation{}, true, ErrConflict
	}
	if err := insertOperationReplayTx(ctx, tx, existing.Receipt.ID, input, now); err != nil {
		return providerops.Operation{}, true, err
	}
	return existing, true, nil
}

func memberPurchaseFactsTx(ctx context.Context, tx *sql.Tx, purchaseID, userID string) (purchaseops.PurchaseFacts, error) {
	var purchase model.Purchase
	var validFrom, validUntil, created string
	var firstTerm int
	err := tx.QueryRowContext(ctx, `SELECT purchases.id,purchases.user_id,combos.name,purchases.charged_txb_minor,
		purchases.core_gross_txb_minor,purchases.status,purchases.valid_from,purchases.valid_until,
		COALESCE(purchases.entitlement_reset_strategy,combos.reset_strategy),purchases.created_at,
		CASE WHEN purchases.renewal_batch_id IS NULL AND purchases.auto_renew_source_purchase_id IS NULL
		AND NOT EXISTS(SELECT 1 FROM purchases successor WHERE successor.auto_renew_source_purchase_id=purchases.id)
		AND NOT EXISTS(SELECT 1 FROM renewal_batches WHERE source_purchase_id=purchases.id) THEN 1 ELSE 0 END
		FROM purchases JOIN combos ON combos.id=purchases.combo_id WHERE purchases.id=? AND purchases.user_id=?`, purchaseID, userID).
		Scan(&purchase.ID, &purchase.UserID, &purchase.ComboName, &purchase.PriceTXBMinor, &purchase.CoreGrossTXBMinor,
			&purchase.Status, &validFrom, &validUntil, &purchase.ResetStrategy, &created, &firstTerm)
	if errors.Is(err, sql.ErrNoRows) {
		return purchaseops.PurchaseFacts{}, ErrNotFound
	}
	if err != nil {
		return purchaseops.PurchaseFacts{}, err
	}
	purchase.Price = model.TXBMoney(purchase.PriceTXBMinor)
	if purchase.ValidFrom, err = parseStamp(validFrom); err == nil {
		purchase.ValidUntil, err = parseStamp(validUntil)
	}
	if err == nil {
		purchase.CreatedAt, err = parseStamp(created)
	}
	return purchaseops.PurchaseFacts{Purchase: purchase, CoreGrossMinor: purchase.CoreGrossTXBMinor, FirstTerm: firstTerm == 1}, err
}

func memberOperationEligible(facts purchaseops.PurchaseFacts, reset bool, now time.Time) bool {
	purchase := facts.Purchase
	if purchase.Status != "active" || now.Before(purchase.ValidFrom) || !now.Before(purchase.ValidUntil) {
		return false
	}
	if reset {
		_, valid := purchaseops.ResetPriceMinor(facts.CoreGrossMinor, purchase.ResetStrategy)
		return valid
	}
	return facts.FirstTerm && !now.Before(purchase.CreatedAt) && now.Before(purchase.CreatedAt.Add(24*time.Hour))
}

func openMemberOperationTx(ctx context.Context, tx *sql.Tx, purchaseID, kind string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations operation
		JOIN provider_operation_items item ON item.operation_id=operation.id
		WHERE operation.kind=? AND item.target_type='purchase' AND item.target_id=?
		AND operation.status IN ('queued','processing','pending_review')`, kind, purchaseID).Scan(&count)
	return count > 0, err
}
