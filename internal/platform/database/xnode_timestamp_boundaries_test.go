package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAccessRepairComparesExactPurchaseInstants(t *testing.T) {
	whole := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, test := range []struct {
		name            string
		from, until, at time.Time
		allowed         bool
	}{
		{"future fractional start", whole.Add(500 * time.Millisecond), whole.Add(time.Hour), whole, false},
		{"whole start fractional now", whole, whole.Add(time.Hour), whole.Add(time.Nanosecond), true},
		{"fractional expiry whole now", whole.Add(-time.Hour), whole.Add(time.Nanosecond), whole, true},
		{"expired whole boundary", whole.Add(-time.Hour), whole, whole.Add(time.Nanosecond), false},
		{"exact expiry", whole.Add(-time.Hour), whole, whole, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, ctx := newPurchasePeriodRestoreFixture(t, "open"), context.Background()
			if _, err := f.store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(test.from), stamp(test.until), f.purchase.ID); err != nil {
				t.Fatal(err)
			}
			purchase, err := f.store.PurchaseByID(ctx, f.purchase.ID)
			if err != nil {
				t.Fatal(err)
			}
			err = f.store.RepairXNodeAccessPeriod(ctx, f.identity, purchase, f.period, 0, test.at, false)
			if test.allowed && err != nil || !test.allowed && !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("repair admitted the wrong paid window", err)
			}
			period, err := f.store.XNodePeriod(ctx, f.period.ID)
			if err != nil || period.UsedBytes != 77 || period.ID != f.period.ID {
				t.Fatal("window check changed accounting evidence", period, err)
			}
		})
	}
}

func TestAccessRepairQueuedContinuityKeepsFractionalExpiry(t *testing.T) {
	f, ctx := newPurchasePeriodRestoreFixture(t, "open"), context.Background()
	queued, err := f.store.CreatePurchase(ctx, PurchaseInput{UserID: f.identity.UserID, ComboID: f.purchase.ComboID, IdempotencyKey: "repair-boundary"}, f.now.Add(time.Minute))
	if err != nil || queued.Status != "queued" {
		t.Fatal(queued, err)
	}
	whole := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	until := whole.Add(time.Nanosecond)
	for _, update := range []struct {
		id          string
		from, until time.Time
	}{
		{f.purchase.ID, whole.Add(-time.Hour), whole},
		{queued.ID, whole, until},
	} {
		if _, err := f.store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(update.from), stamp(update.until), update.id); err != nil {
			t.Fatal(err)
		}
	}
	projection, err := f.store.ContinuityEntitlement(ctx, queued.ID, whole)
	if err != nil || projection == nil {
		t.Fatal(projection, err)
	}
	if err = f.store.RepairXNodeAccessPeriod(ctx, f.identity, *projection, f.period, 0, whole, false); err != nil {
		t.Fatal("paid queued continuity ended early", err)
	}
	if err = f.store.RepairXNodeAccessPeriod(ctx, f.identity, *projection, f.period, 0, until, false); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("expired queued continuity reopened access", err)
	}
}

func TestIdentityGrowthUsesInclusiveExactInstants(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	whole := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	times := []string{
		stamp(whole.Add(-time.Nanosecond)), stamp(whole), stamp(whole.Add(250 * time.Millisecond)),
		"2030-01-02T03:04:05.500000000Z", stamp(whole.Add(500*time.Millisecond + time.Nanosecond)),
	}
	for index, at := range times {
		user := createTestUser(t, s, int64(95000+index))
		identity, err := s.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, CredentialsCipher: fmt.Sprint("growth-", index)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_identities SET created_at=$1 WHERE proxy_user_id=$2`, at, identity.ProxyUserID); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		from, through time.Time
		count         int64
	}{
		{whole, whole.Add(500 * time.Millisecond), 3},
		{whole, whole, 1},
		{whole.Add(500 * time.Millisecond), whole.Add(500 * time.Millisecond), 1},
		{whole.Add(time.Nanosecond), whole.Add(499999999 * time.Nanosecond), 1},
	} {
		count, err := s.XNodeIdentityGrowth(ctx, test.from, test.through)
		if err != nil || count != test.count {
			t.Fatal("growth boundary mismatch", test, count, err)
		}
	}
}

func TestAccessRepairSchedulerKeepsExactBlockedOwnerBenefits(t *testing.T) {
	whole := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, test := range []struct {
		name        string
		from, until time.Time
		want        int
	}{
		{"last paid nanosecond", whole.Add(-time.Hour), whole.Add(time.Nanosecond), 1},
		{"not started", whole.Add(time.Nanosecond), whole.Add(time.Hour), 0},
		{"exact expiry", whole.Add(-time.Hour), whole, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, ctx := newPurchasePeriodRestoreFixture(t, "closing"), context.Background()
			// Existing complete credentials permit paid-period recovery even if
			// Telegram delivery is blocked; no new credentials are provisioned.
			if _, err := f.store.db.ExecContext(ctx, `UPDATE users SET bot_blocked=1 WHERE id=$1`, f.identity.UserID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.db.ExecContext(ctx, `UPDATE xnode_identities SET subscription_cipher='retained',subscription_hash=$1 WHERE proxy_user_id=$2`, make([]byte, 32), f.identity.ProxyUserID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.db.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(test.from), stamp(test.until), f.purchase.ID); err != nil {
				t.Fatal(err)
			}
			if err := f.store.EnqueueXNodeAccessRepairs(ctx, whole); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := f.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1 AND payload::jsonb->>'userId'=$2`, xnode.AccessRepairJob, f.identity.UserID).Scan(&count); err != nil || count != test.want {
				t.Fatal("repair scheduler changed exact paid benefit", count, test.want, err)
			}
		})
	}
}
