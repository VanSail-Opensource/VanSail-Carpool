package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"time"
)

func hasRetiredResetReward(draw activity.LuckyDraw) bool {
	for _, prize := range draw.Prizes {
		if prize.Reward.Kind == activity.RewardTrafficReset {
			return true
		}
	}
	return false
}

// Parent row ordering serializes joins, awards and retirement before wallet changes.
func lockedLuckyDrawTx(ctx context.Context, tx *sql.Tx, id string, enabledOnly bool) (activity.LuckyDraw, error) {
	query := luckyDrawSelect + ` WHERE id=$1`
	if enabledOnly {
		query += ` AND kind='instant' AND status='open'`
	}
	return scanLuckyDraw(tx.QueryRowContext(ctx, query+` FOR UPDATE`, id))
}

// Pending raffle fees and reservations are refunded through the existing exact
// ledger helper. Completed awards and entitlement jobs are immutable evidence.
func retireResetDrawTx(ctx context.Context, tx *sql.Tx, draw *activity.LuckyDraw, now time.Time) error {
	if draw.Status == "cancelled" || draw.Status == "completed" {
		return nil
	}
	// Claim cancellation before touching wallets. This row lock also fences a
	// concurrent join, settlement or duplicate maintenance invocation.
	err := tx.QueryRowContext(ctx, `UPDATE activity_lucky_draws SET status='cancelled',updated_at=$1 WHERE id=$2 AND status NOT IN ('cancelled','completed') RETURNING kind,name`, stamp(now), draw.ID).Scan(&draw.Kind, &draw.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.QueryRowContext(ctx, `SELECT status FROM activity_lucky_draws WHERE id=$1`, draw.ID).Scan(&draw.Status)
	}
	if err != nil {
		return err
	}
	draw.Status = "cancelled"
	tickets, err := activeRaffleTicketsTx(ctx, tx, draw.ID)
	if err != nil {
		return err
	}
	for _, ticket := range tickets {
		if err = refundRaffleTicketTx(ctx, tx, ticket, draw.Name, now); err != nil {
			return err
		}
	}
	if draw.Kind == "raffle" {
		payload, _ := json.Marshal(map[string]string{"drawId": draw.ID})
		if err = insertOutboxTx(ctx, tx, "draw_telegram_delete", string(payload), now, now); err != nil {
			return err
		}
	}
	audit, err := ids.New()
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, audit, nil, "retirement.traffic_reset_draw", "lucky_draw", draw.ID, `{}`, now)
}

// Backup-gated daily maintenance retires untouched legacy definitions in bounded
// batches. New joins and settlement perform the same retirement transaction.
func retireResetDrawsTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT d.id FROM activity_lucky_draws d WHERE d.status NOT IN ('cancelled','completed') AND EXISTS(SELECT 1 FROM activity_lucky_prizes p WHERE p.draw_id=d.id AND (p.reward_payload::jsonb)->>'kind'='traffic_reset') ORDER BY d.id LIMIT 20 FOR UPDATE OF d SKIP LOCKED`)
	if err != nil {
		return err
	}
	pending := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range pending {
		draw, err := luckyDrawByID(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if err = retireResetDrawTx(ctx, tx, &draw, now); err != nil {
			return err
		}
		if draw.Status == "cancelled" {
			counts["retired_reset_draws"]++
		}
	}
	return nil
}
