package database

import (
	"context"
	"database/sql"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// SetAccountCleanupBot records queued getMe verification. Zero disables cleanup
// without converting an outage into failed-account evidence or changing epoch.
func (s *Store) SetAccountCleanupBot(ctx context.Context, id int64, at time.Time) (model.AccountCleanupHealth, error) {
	if id < 0 || id >= 1<<52 || at.IsZero() {
		return model.AccountCleanupHealth{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.AccountCleanupHealth, error) {
		h, err := lockCleanupHealth(ctx, tx, true)
		if err != nil {
			return h, err
		}
		if id == 0 {
			_, err = tx.ExecContext(ctx, `UPDATE account_cleanup_health SET verified=0 WHERE singleton=1`)
		} else {
			changed := id != h.BotID
			if changed && h.Epoch == math.MaxInt64 {
				return h, ErrConflict
			}
			_, err = tx.ExecContext(ctx, `UPDATE account_cleanup_health SET bot_id=$1,bot_epoch=bot_epoch+$2,verified=1,verified_at=$3,window_start=CASE WHEN $2=1 THEN NULL ELSE window_start END,checked_count=CASE WHEN $2=1 THEN 0 ELSE checked_count END,unhealthy_count=CASE WHEN $2=1 THEN 0 ELSE unhealthy_count END,paused_until=CASE WHEN $2=1 THEN NULL ELSE paused_until END WHERE singleton=1`, id, boolInt(changed), at.UTC())
		}
		if err != nil {
			return h, err
		}
		return lockCleanupHealth(ctx, tx, true)
	})
}

// A failure surge pauses admission durably. Retried nonce completions cannot
// count twice; completed probes are spaced daily for each account.
func recordCleanupHealthTx(ctx context.Context, tx *sql.Tx, degraded bool, at time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE account_cleanup_health SET
 checked_count=CASE WHEN window_start IS NULL OR window_start<=$1 THEN 1 ELSE checked_count+1 END,
 unhealthy_count=CASE WHEN window_start IS NULL OR window_start<=$1 THEN $2 ELSE unhealthy_count+$2 END,
 window_start=CASE WHEN window_start IS NULL OR window_start<=$1 THEN $3 ELSE window_start END
 WHERE singleton=1`, at.Add(-10*time.Minute), boolInt(degraded), at)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE account_cleanup_health SET paused_until=$1
 WHERE singleton=1 AND checked_count>=5 AND unhealthy_count>=3 AND unhealthy_count::numeric*2>=checked_count
 AND (paused_until IS NULL OR paused_until<$1)`, at.Add(24*time.Hour))
	return err
}
