package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func notificationNativeUser(t *testing.T, s *Store, telegram int64) (xnode.Identity, xnode.Period) {
	t.Helper()
	ctx, now := context.Background(), time.Now().UTC().Add(-time.Minute)
	user := createTestUser(t, s, telegram)
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET onboarding_state='complete',proxy_user_id=NULL WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdjustBalance(ctx, user.ID, 1000, fmt.Sprint("notification-native-", telegram), "fixture", now); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, s, fmt.Sprint("Native ", telegram), 100, 30)
	purchase, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: fmt.Sprint("native-purchase-", telegram)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	identity, err := s.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, CredentialsCipher: "native-credentials"})
	if err != nil {
		t.Fatal(err)
	}
	reset := now.Add(-time.Hour)
	period := xnode.Period{ID: fmt.Sprint("native-period-", telegram), ProxyUserID: identity.ProxyUserID, PurchaseID: purchase.ID,
		StartedAt: now, LimitBytes: 1000, Strategy: "DAY", LastResetAt: &reset, Status: "open"}
	if err = s.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=901 WHERE id=$1`, period.ID); err != nil {
		t.Fatal(err)
	}
	return identity, period
}

func TestXNodeNotificationUsersPagesNativeOnlyOwners(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	first, firstPeriod := notificationNativeUser(t, s, 88401)
	second, secondPeriod := notificationNativeUser(t, s, 88402)
	rows, err := s.XNodeNotificationUsers(ctx, 0, 1)
	if err != nil || len(rows) != 1 || rows[0].ID != first.ProxyUserID || rows[0].OwnerID != first.UserID ||
		rows[0].AccountGeneration != 0 || rows[0].PeriodID != firstPeriod.ID || rows[0].UsedBytes != 901 ||
		rows[0].LimitBytes != 1000 || rows[0].ResetStrategy != "DAY" || rows[0].LastTrafficResetAt == nil ||
		!rows[0].LastTrafficResetAt.Equal(*firstPeriod.LastResetAt) {
		t.Fatal("native-only first page lost owner or current-period scope", rows, err)
	}
	rows, err = s.XNodeNotificationUsers(ctx, first.ProxyUserID, 1001)
	if err != nil || len(rows) != 1 || rows[0].ID != second.ProxyUserID || rows[0].PeriodID != secondPeriod.ID {
		t.Fatal("keyset page repeated or skipped native owner", rows, err)
	}
	rows, err = s.XNodeNotificationUsers(ctx, second.ProxyUserID, 1)
	if err != nil || len(rows) != 0 {
		t.Fatal("terminal native page was not empty", rows, err)
	}
	for _, request := range []struct {
		after int64
		limit int
	}{{-1, 1}, {0, 0}, {0, 1002}} {
		if _, err = s.XNodeNotificationUsers(ctx, request.after, request.limit); !errors.Is(err, xnode.ErrInvalid) {
			t.Fatal("unbounded or invalid cursor accepted", request, err)
		}
	}
}

func TestXNodeNotificationUsersExcludeRetiredAndForeignPeriods(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	identity, period := notificationNativeUser(t, s, 88403)
	_, foreignPeriod := notificationNativeUser(t, s, 88404)
	cases := []struct {
		name, change, restore   string
		changeArgs, restoreArgs []any
	}{
		{"purging owner", `UPDATE users SET account_status='purging' WHERE id=$1`, `UPDATE users SET account_status='active' WHERE id=$1`, []any{identity.UserID}, []any{identity.UserID}},
		{"incomplete owner", `UPDATE users SET onboarding_state='intro' WHERE id=$1`, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, []any{identity.UserID}, []any{identity.UserID}},
		{"retired credentials", `UPDATE xnode_identities SET credentials_cipher='' WHERE user_id=$1`, `UPDATE xnode_identities SET credentials_cipher='native-credentials' WHERE user_id=$1`, []any{identity.UserID}, []any{identity.UserID}},
		{"disabled identity", `UPDATE xnode_identities SET access_disabled=1 WHERE user_id=$1`, `UPDATE xnode_identities SET access_disabled=0 WHERE user_id=$1`, []any{identity.UserID}, []any{identity.UserID}},
		{"closed period", `UPDATE xnode_periods SET status='closed' WHERE id=$1`, `UPDATE xnode_periods SET status='open' WHERE id=$1`, []any{period.ID}, []any{period.ID}},
		{"retired purchase", `UPDATE purchases SET status='cancelled' WHERE id=$1`, `UPDATE purchases SET status='active' WHERE id=$1`, []any{period.PurchaseID}, []any{period.PurchaseID}},
		{"foreign current period", `UPDATE xnode_identities SET current_period_id=$2 WHERE user_id=$1`, `UPDATE xnode_identities SET current_period_id=$2 WHERE user_id=$1`, []any{identity.UserID, foreignPeriod.ID}, []any{identity.UserID, period.ID}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := s.db.ExecContext(ctx, test.change, test.changeArgs...); err != nil {
				t.Fatal(err)
			}
			rows, err := s.XNodeNotificationUsers(ctx, identity.ProxyUserID-1, 1001)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.ID == identity.ProxyUserID {
					t.Fatal("retired/foreign scope entered notification page", row)
				}
			}
			if _, err = s.db.ExecContext(ctx, test.restore, test.restoreArgs...); err != nil {
				t.Fatal(err)
			}
		})
	}
}
