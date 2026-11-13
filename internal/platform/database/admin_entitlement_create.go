package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// ErrEntitlementOverlap reports a manual term that intersects an existing
// active, activating, or queued purchase for the same owner.
var ErrEntitlementOverlap = errors.New("entitlement term overlaps an existing entitlement")

// adminGrantKeyPrefix keeps manual grants out of the member purchase
// idempotency namespace and scopes each command to its administrator actor.
const adminGrantKeyPrefix = "admin-grant:"

// AdminEntitlementCreateInput describes one validated zero-charge fixed-term
// grant. It never moves TXB and never creates ledger debt.
type AdminEntitlementCreateInput struct {
	ActorUserID, UserID                        string
	IdempotencyKey, RequestFingerprint, Reason string
	ComboID, ResetStrategy                     string
	ValidFrom, ValidUntil                      time.Time
	TrafficLimitBytes                          int64
	SquadUUIDs                                 []string
}

// CreateAdminEntitlement inserts a zero-charge purchase, its immutable
// entitlement snapshot, an actor audit, a durable user sync, and one gated
// member notification in a single transaction.
func (s *Store) CreateAdminEntitlement(ctx context.Context, input AdminEntitlementCreateInput, now time.Time) (model.Purchase, error) {
	now = now.UTC()
	for retry := 0; ; retry++ {
		purchase, err := s.createAdminEntitlementAttempt(ctx, input, now)
		if !s.retryTransaction(ctx, err, retry) {
			return purchase, err
		}
	}
}

func (s *Store) createAdminEntitlementAttempt(ctx context.Context, input AdminEntitlementCreateInput, now time.Time) (model.Purchase, error) {
	if !validAdminEntitlementCreate(input) {
		return model.Purchase{}, ErrConflict
	}
	squads := append([]string{}, input.SquadUUIDs...)
	encodedSquads, err := json.Marshal(squads)
	if err != nil {
		return model.Purchase{}, fmt.Errorf("encode entitlement squads: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if replay, found, err := adminEntitlementGrantReplayTx(ctx, tx, input.ActorUserID, input.UserID, input.IdempotencyKey); found || err != nil {
		if err != nil {
			return model.Purchase{}, err
		}
		if replay.RequestFingerprint != input.RequestFingerprint {
			return model.Purchase{}, ErrConflict
		}
		return replay.Purchase, nil
	}
	if err := catalogAdminActorTx(ctx, tx, input.ActorUserID); err != nil {
		return model.Purchase{}, err
	}
	if err := lockProviderOperationAccountsTx(ctx, tx, input.ActorUserID, input.UserID); err != nil {
		return model.Purchase{}, err
	}
	if err := lockPurchaseOwnerTx(ctx, tx, input.UserID); err != nil {
		return model.Purchase{}, err
	}
	// Recheck after owner serialization before mutable catalog/overlap admission.
	// A serializable snapshot may still predate a concurrent commit; insertion
	// conflicts below recover that accepted result from a fresh transaction.
	if replay, found, err := adminEntitlementGrantReplayTx(ctx, tx, input.ActorUserID, input.UserID, input.IdempotencyKey); found || err != nil {
		if err != nil {
			return model.Purchase{}, err
		}
		if replay.RequestFingerprint != input.RequestFingerprint {
			return model.Purchase{}, ErrConflict
		}
		return replay.Purchase, nil
	}
	idempotencyKey := adminGrantKeyPrefix + input.ActorUserID + ":" + input.IdempotencyKey
	// Replays return above, so an accepted grant stays immutable even if the
	// owner later starts hourly access. New grants cannot double-cover an owner
	// who already holds a preparing or active hourly enrollment.
	if err := rejectHourlyLegacyPurchaseTx(ctx, tx, input.UserID); err != nil {
		return model.Purchase{}, err
	}
	var comboName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM combos WHERE id=$1 AND active=1 FOR SHARE`, input.ComboID).Scan(&comboName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Purchase{}, ErrNotFound
		}
		return model.Purchase{}, err
	}
	for _, squad := range squads {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_groups WHERE id=$1)`, squad).Scan(&exists); err != nil {
			return model.Purchase{}, err
		}
		if !exists {
			return model.Purchase{}, ErrNotFound
		}
	}
	var overlap bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=$1
		AND status IN ('activating','active','queued') AND `+purchaseTimestampOrderSQL("valid_from")+`<$2 AND `+purchaseTimestampOrderSQL("valid_until")+`>$3)`,
		input.UserID, purchaseTimestampOrder(input.ValidUntil), purchaseTimestampOrder(input.ValidFrom)).Scan(&overlap); err != nil {
		return model.Purchase{}, err
	}
	if overlap {
		return model.Purchase{}, ErrEntitlementOverlap
	}
	purchaseID, err := ids.New()
	if err != nil {
		return model.Purchase{}, err
	}
	status := "queued"
	if !input.ValidFrom.After(now) {
		status = "active"
	}
	termSeconds, err := adminEntitlementTermSeconds(input.ValidFrom, input.ValidUntil)
	if err != nil {
		return model.Purchase{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,
		coupon_grant_id,gross_price_txb_minor,core_gross_txb_minor,coupon_discount_txb_minor,auto_renew_enabled,
		recurring_discount_attached,idempotency_key,request_fingerprint,created_at,updated_at,purchased_term_seconds,
		entitlement_traffic_limit_bytes,entitlement_reset_strategy,entitlement_squad_uuids)
		VALUES($1,$2,$3,0,$4,$5,$6,NULL,0,0,0,0,0,$7,$8,$9,$9,$10,$11,$12,$13)`,
		purchaseID, input.UserID, input.ComboID, stamp(input.ValidFrom), stamp(input.ValidUntil), status,
		idempotencyKey, input.RequestFingerprint, stamp(now), termSeconds, input.TrafficLimitBytes,
		input.ResetStrategy, string(encodedSquads))
	if err != nil {
		err = fmt.Errorf("insert manual entitlement: %w", err)
		if !isUniqueViolation(err) {
			return model.Purchase{}, err
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return model.Purchase{}, errors.Join(err, rollbackErr)
		}
		return s.replayManualGrantInsertConflict(ctx, input, err)
	}
	if err := insertManualEntitlementAudit(ctx, tx, input.ActorUserID, purchaseID, input, comboName, status, now); err != nil {
		return model.Purchase{}, err
	}
	syncPayload, err := json.Marshal(map[string]string{"userId": input.UserID})
	if err != nil {
		return model.Purchase{}, err
	}
	if err := insertOutboxTx(ctx, tx, "xnode_sync_user", string(syncPayload), now, now); err != nil {
		return model.Purchase{}, err
	}
	facts := adminNotificationBase("entitlement_edit", input.Reason, now)
	facts[notifications.FactCombo] = comboName
	if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+purchaseID+":"+input.UserID, input.UserID,
		jobpayload.UserEventAdminUpdate, userSyncGate(input.UserID), facts, now); err != nil {
		return model.Purchase{}, err
	}
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1`, purchaseID))
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.SquadUUIDs, err = purchaseSquadsFrom(ctx, tx, purchaseID); err != nil {
		return model.Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, err
	}
	return purchase, nil
}
