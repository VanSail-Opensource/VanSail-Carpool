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
	if err := lockPurchaseOwnerTx(ctx, tx, input.UserID); err != nil {
		return model.Purchase{}, err
	}
	idempotencyKey := adminGrantKeyPrefix + input.ActorUserID + ":" + input.IdempotencyKey
	var existingID, existingFingerprint string
	loadErr := tx.QueryRowContext(ctx, `SELECT id,request_fingerprint FROM purchases WHERE user_id=$1 AND idempotency_key=$2`,
		input.UserID, idempotencyKey).Scan(&existingID, &existingFingerprint)
	if loadErr == nil {
		if existingFingerprint != input.RequestFingerprint {
			return model.Purchase{}, ErrConflict
		}
		if err := tx.Rollback(); err != nil {
			return model.Purchase{}, err
		}
		return s.PurchaseByID(ctx, existingID)
	}
	if !errors.Is(loadErr, sql.ErrNoRows) {
		return model.Purchase{}, fmt.Errorf("load manual entitlement idempotency key: %w", loadErr)
	}
	// Replays return above, so an accepted grant stays immutable even if the
	// owner later starts hourly access. New grants cannot double-cover an owner
	// who already holds a preparing or active hourly enrollment.
	if err := rejectHourlyLegacyPurchaseTx(ctx, tx, input.UserID); err != nil {
		return model.Purchase{}, err
	}
	var comboName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM combos WHERE id=$1 AND active=1`, input.ComboID).Scan(&comboName); err != nil {
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
		AND status IN ('activating','active','queued') AND valid_from::timestamptz<$2 AND valid_until::timestamptz>$3)`,
		input.UserID, stamp(input.ValidUntil), stamp(input.ValidFrom)).Scan(&overlap); err != nil {
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
	termSeconds := int64(input.ValidUntil.Sub(input.ValidFrom) / time.Second)
	_, err = tx.ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,
		coupon_grant_id,gross_price_txb_minor,core_gross_txb_minor,coupon_discount_txb_minor,auto_renew_enabled,
		recurring_discount_attached,idempotency_key,request_fingerprint,created_at,updated_at,purchased_term_seconds,
		entitlement_traffic_limit_bytes,entitlement_reset_strategy,entitlement_squad_uuids)
		VALUES($1,$2,$3,0,$4,$5,$6,NULL,0,0,0,0,0,$7,$8,$9,$9,$10,$11,$12,$13)`,
		purchaseID, input.UserID, input.ComboID, stamp(input.ValidFrom), stamp(input.ValidUntil), status,
		idempotencyKey, input.RequestFingerprint, stamp(now), termSeconds, input.TrafficLimitBytes,
		input.ResetStrategy, string(encodedSquads))
	if err != nil {
		return model.Purchase{}, fmt.Errorf("insert manual entitlement: %w", err)
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
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, err
	}
	return s.PurchaseByID(ctx, purchaseID)
}

func validAdminEntitlementCreate(input AdminEntitlementCreateInput) bool {
	if input.ActorUserID == "" || input.UserID == "" || input.ComboID == "" || input.IdempotencyKey == "" ||
		len(input.IdempotencyKey) > 128 || input.TrafficLimitBytes <= 0 ||
		(input.ResetStrategy != "DAY" && input.ResetStrategy != "WEEK" && input.ResetStrategy != "MONTH_ROLLING") ||
		input.ValidUntil.Sub(input.ValidFrom) < time.Second || len(input.Reason) < 3 || len(input.Reason) > 500 {
		return false
	}
	return true
}

// lockPurchaseOwnerTx serializes grants with member checkout and other grants,
// then requires a durable owner.
func lockPurchaseOwnerTx(ctx context.Context, tx *sql.Tx, userID string) error {
	var accountGeneration int64
	if err := tx.QueryRowContext(ctx, `SELECT account_generation FROM users WHERE id=$1 FOR NO KEY UPDATE`, userID).
		Scan(&accountGeneration); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func insertManualEntitlementAudit(ctx context.Context, tx *sql.Tx, actorID, purchaseID string,
	input AdminEntitlementCreateInput, comboName, status string, now time.Time) error {
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]any{
		"reason": input.Reason, "txbMovementMinor": 0,
		"after": map[string]any{"comboId": input.ComboID, "comboName": comboName,
			"validFrom": input.ValidFrom, "validUntil": input.ValidUntil, "status": status,
			"trafficLimitBytes": input.TrafficLimitBytes, "resetStrategy": input.ResetStrategy,
			"squadUuids": input.SquadUUIDs},
	})
	if err != nil {
		return fmt.Errorf("encode manual entitlement audit: %w", err)
	}
	return insertAuditTx(ctx, tx, auditID, &actorID, "entitlement.create", "purchase", purchaseID, string(detail), now)
}
