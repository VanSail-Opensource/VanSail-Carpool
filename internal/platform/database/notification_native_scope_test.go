package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
)

func TestNativeTrafficNoticeRejectsStaleLifecycleAndRechecksCounters(t *testing.T) {
	for _, scenario := range []string{"owner", "generation", "period", "credentials", "access", "counter"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			identity, period := notificationNativeUser(t, store, 88405)
			user := notifications.TrafficUser{ID: identity.ProxyUserID, OwnerID: identity.UserID, PeriodID: period.ID, UsedBytes: 901, LimitBytes: 1000}
			var query string
			var args []any
			switch scenario {
			case "owner":
				query, args = `UPDATE users SET account_status='purging' WHERE id=$1`, []any{identity.UserID}
			case "generation":
				query, args = `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, []any{identity.UserID}
			case "period":
				user.PeriodID = "retired-captured-period"
			case "credentials":
				query, args = `UPDATE xnode_identities SET credentials_cipher='' WHERE proxy_user_id=$1`, []any{identity.ProxyUserID}
			case "access":
				query, args = `UPDATE xnode_identities SET access_disabled=1 WHERE proxy_user_id=$1`, []any{identity.ProxyUserID}
			case "counter":
				query, args = `UPDATE xnode_periods SET used_bytes=900 WHERE id=$1`, []any{period.ID}
			}
			if query != "" {
				if _, err := store.DB().ExecContext(ctx, query, args...); err != nil {
					t.Fatal(err)
				}
			}
			if inserted, err := store.EnqueueTrafficThresholdNotification(ctx, user, time.Now().UTC()); err != nil || inserted {
				t.Fatalf("stale native scope emitted a warning: %v, %v", inserted, err)
			}
			var count int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE source_kind='traffic-90'`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("stale native scope retained a warning: %d, %v", count, err)
			}
		})
	}
}
