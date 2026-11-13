package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func validAdminEntitlementCreate(input AdminEntitlementCreateInput) bool {
	if input.ActorUserID == "" || input.UserID == "" || input.ComboID == "" || input.IdempotencyKey == "" ||
		len(input.IdempotencyKey) > 128 || input.TrafficLimitBytes <= 0 ||
		(input.ResetStrategy != "DAY" && input.ResetStrategy != "WEEK" && input.ResetStrategy != "MONTH_ROLLING") ||
		len(input.Reason) < 3 || len(input.Reason) > 500 {
		return false
	}
	_, err := adminEntitlementTermSeconds(input.ValidFrom, input.ValidUntil)
	return err == nil
}

// Accepted term evidence must not saturate at time.Duration's 292-year limit.
// Canonical RFC3339 years keep this second difference well within int64.
func adminEntitlementTermSeconds(from, until time.Time) (int64, error) {
	if from.UTC().Year() < 1 || from.UTC().Year() > 9999 || until.UTC().Year() < 1 || until.UTC().Year() > 9999 || !until.After(from) {
		return 0, ErrConflict
	}
	seconds := until.Unix() - from.Unix()
	if until.Nanosecond() < from.Nanosecond() {
		seconds--
	}
	if seconds < 1 {
		return 0, ErrConflict
	}
	return seconds, nil
}

// Take each account's strongest required lock in ID order before shared
// lifecycle validation. Taking owner SHARE first makes concurrent grants all
// wait to upgrade each other's lock, including when the actor is the owner.
// A distinct actor stays shared so grants to unrelated owners can proceed.
func lockAdminEntitlementAccountsTx(ctx context.Context, tx *sql.Tx, actorID, ownerID string) error {
	if actorID < ownerID {
		if err := catalogAdminActorTx(ctx, tx, actorID); err != nil {
			return err
		}
		if err := lockPurchaseOwnerTx(ctx, tx, ownerID); err != nil {
			return err
		}
	} else {
		if err := lockPurchaseOwnerTx(ctx, tx, ownerID); err != nil {
			return err
		}
		if err := catalogAdminActorTx(ctx, tx, actorID); err != nil {
			return err
		}
	}
	return lockProviderOperationAccountsTx(ctx, tx, actorID, ownerID)
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
