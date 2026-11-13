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
