package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func compactActivityTx(ctx context.Context, tx *sql.Tx, cutoff7Days, cutoff24Hours, now time.Time, counts map[string]int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO activity_daily_rollups(local_date,checkin_count,
		checkin_reward_txb_minor,group_message_count,group_message_reward_txb_minor,updated_at)
		SELECT local_date,COUNT(*),COALESCE(SUM(reward_minor),0),0,0,? FROM activity_daily_checkins
		WHERE created_at<? GROUP BY local_date
		ON CONFLICT(local_date) DO UPDATE SET checkin_count=activity_daily_rollups.checkin_count+excluded.checkin_count,
		checkin_reward_txb_minor=activity_daily_rollups.checkin_reward_txb_minor+excluded.checkin_reward_txb_minor,
		updated_at=excluded.updated_at`, stamp(now), stamp(cutoff7Days))
	if err != nil {
		return fmt.Errorf("roll up daily check-ins: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_daily_rollups(local_date,checkin_count,
		checkin_reward_txb_minor,group_message_count,group_message_reward_txb_minor,updated_at)
		SELECT local_date,0,0,COUNT(*),0,? FROM activity_group_message_raw_events
		WHERE created_at<? GROUP BY local_date
		ON CONFLICT(local_date) DO UPDATE SET group_message_count=activity_daily_rollups.group_message_count+excluded.group_message_count,
		updated_at=excluded.updated_at`, stamp(now), stamp(cutoff24Hours))
	if err != nil {
		return fmt.Errorf("roll up raw group messages: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_daily_rollups(local_date,checkin_count,
		checkin_reward_txb_minor,group_message_count,group_message_reward_txb_minor,updated_at)
		SELECT window.local_date,0,0,0,
			COALESCE(SUM((SELECT COALESCE(SUM(ledger.delta_txb_minor),0) FROM ledger_entries ledger
				WHERE ledger.user_id=window.user_id AND ledger.kind='activity_group_message_reward'
				AND ledger.note=window.local_date)),0),?
		FROM activity_group_message_windows window WHERE window.updated_at<? GROUP BY window.local_date
		ON CONFLICT(local_date) DO UPDATE SET
		group_message_reward_txb_minor=activity_daily_rollups.group_message_reward_txb_minor+excluded.group_message_reward_txb_minor,
		updated_at=excluded.updated_at`, stamp(now), stamp(cutoff24Hours))
	if err != nil {
		return fmt.Errorf("roll up group messages: %w", err)
	}
	if counts["daily_checkins"], err = deleteCount(ctx, tx,
		`DELETE FROM activity_daily_checkins WHERE created_at<?`, stamp(cutoff7Days)); err != nil {
		return fmt.Errorf("prune daily check-ins: %w", err)
	}
	if counts["group_message_windows"], err = deleteCount(ctx, tx,
		`DELETE FROM activity_group_message_windows WHERE updated_at<?`, stamp(cutoff24Hours)); err != nil {
		return fmt.Errorf("prune group-message windows: %w", err)
	}
	if counts["group_message_facts"], err = deleteCount(ctx, tx,
		`DELETE FROM activity_group_message_raw_events WHERE created_at<?`, stamp(cutoff24Hours)); err != nil {
		return fmt.Errorf("prune raw group-message facts: %w", err)
	}
	if counts["group_message_events"], err = deleteCount(ctx, tx,
		`DELETE FROM activity_group_message_events WHERE created_at<?`, stamp(cutoff24Hours)); err != nil {
		return fmt.Errorf("prune group-message events: %w", err)
	}
	return nil
}
