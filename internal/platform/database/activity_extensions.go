package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"math"
	"time"
)

func applySubscriptionExtensionTx(ctx context.Context, tx *sql.Tx, userID string, days int, sourceType, sourceID string, now time.Time) error {
	if days < 1 || days > 3650 {
		return activity.ErrInvalidInput
	}
	return applySubscriptionExtensionHoursTx(ctx, tx, userID, days*24, sourceType, sourceID, now)
}

func applySubscriptionExtensionHoursTx(ctx context.Context, tx *sql.Tx, userID string, hours int, sourceType, sourceID string, now time.Time) error {
	if hours < 1 || hours > 3650*24 {
		return activity.ErrInvalidInput
	}
	var purchaseID, validUntilRaw string
	err := tx.QueryRowContext(ctx, `SELECT id,valid_until FROM purchases WHERE user_id=$1 AND status IN ('activating','active') AND valid_from<=$2 AND valid_until>$3
		ORDER BY valid_from DESC LIMIT 1`, userID, stamp(now), stamp(now)).Scan(&purchaseID, &validUntilRaw)
	if errors.Is(err, sql.ErrNoRows) {
		creditID, idErr := ids.New()
		if idErr != nil {
			return idErr
		}
		_, insertErr := tx.ExecContext(ctx, `INSERT INTO activity_extension_credits(id,user_id,days,hours,source_type,source_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			creditID, userID, (hours+23)/24, hours, sourceType, sourceID, stamp(now))
		return insertErr
	}
	if err != nil {
		return err
	}
	validUntil, err := parseStamp(validUntilRaw)
	if err != nil {
		return err
	}
	shiftedUntil, err := addSubscriptionHours(validUntil, hours)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_until=$1,updated_at=$2 WHERE id=$3`, stamp(shiftedUntil), stamp(now), purchaseID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,valid_from,valid_until FROM purchases WHERE user_id=$1 AND status='queued' AND valid_from>=$2 ORDER BY valid_from`, userID, validUntilRaw)
	if err != nil {
		return err
	}
	type queuedTerm struct{ id, from, until string }
	queued := make([]queuedTerm, 0)
	for rows.Next() {
		var term queuedTerm
		if err := rows.Scan(&term.id, &term.from, &term.until); err != nil {
			_ = rows.Close()
			return err
		}
		queued = append(queued, term)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, term := range queued {
		from, parseErr := parseStamp(term.from)
		if parseErr != nil {
			return parseErr
		}
		until, parseErr := parseStamp(term.until)
		if parseErr != nil {
			return parseErr
		}
		shiftedFrom, shiftErr := addSubscriptionHours(from, hours)
		if shiftErr != nil {
			return shiftErr
		}
		shiftedUntil, shiftErr := addSubscriptionHours(until, hours)
		if shiftErr != nil {
			return shiftErr
		}
		if _, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2,updated_at=$3 WHERE id=$4`, stamp(shiftedFrom), stamp(shiftedUntil), stamp(now), term.id); err != nil {
			return err
		}
	}
	return insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now)
}

// consumePendingExtensionsTx marks stored extension credits used by the term
// that is actually becoming active. Queued purchases must not consume credits
// early because an older queued term remains the member's next activation.

func consumePendingExtensionsTx(ctx context.Context, tx *sql.Tx, userID, purchaseID string, now time.Time) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,COALESCE(hours,days*24) FROM activity_extension_credits WHERE user_id=$1 AND consumed_at IS NULL ORDER BY created_at,id`, userID)
	if err != nil {
		return 0, err
	}
	idsToConsume := make([]string, 0)
	total := int64(0)
	for rows.Next() {
		var id string
		var days int
		if err := rows.Scan(&id, &days); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if total > math.MaxInt64-int64(days) {
			_ = rows.Close()
			return 0, activity.ErrInvalidInput
		}
		total += int64(days)
		idsToConsume = append(idsToConsume, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()
	for _, creditID := range idsToConsume {
		if _, err := tx.ExecContext(ctx, `UPDATE activity_extension_credits SET consumed_at=$1,consumed_by_purchase_id=$2 WHERE id=$3 AND consumed_at IS NULL`, stamp(now), purchaseID, creditID); err != nil {
			return 0, err
		}
	}
	if total > int64(^uint(0)>>1) {
		return 0, activity.ErrInvalidInput
	}
	return int(total), nil
}

// applyPendingExtensionsToActivationTx applies credits saved while no term was
// active to the exact next activating term, then delays every following queued
// term so subscription periods remain contiguous and non-overlapping.
