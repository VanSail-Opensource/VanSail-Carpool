package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPurchasePeriodRestorePreservesAccountingAndResetOrigin(t *testing.T) {
	for _, status := range []string{"open", "closing", "closed"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			f, ctx := newPurchasePeriodRestoreFixture(t, status), context.Background()
			if _, err := f.store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_traffic_limit_bytes=2000,entitlement_reset_strategy='WEEK',updated_at=$1 WHERE id=$2`, stamp(f.now.Add(time.Second)), f.purchase.ID); err != nil {
				t.Fatal(err)
			}
			purchase, err := f.store.PurchaseByID(ctx, f.purchase.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, purchase, 0, f.now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			actual, err := f.store.XNodePeriod(ctx, f.period.ID)
			if err != nil || actual.Status != "open" || actual.UsedBytes != 77 || actual.LimitBytes != 2000 || actual.Strategy != "WEEK" || actual.EndedAt != nil ||
				!actual.StartedAt.Equal(f.period.StartedAt) || actual.LastResetAt == nil || !actual.LastResetAt.Equal(*f.period.LastResetAt) || actual.ClosureGeneration != f.period.ClosureGeneration {
				t.Fatal("restore replaced accounting", actual, err)
			}
			var count int
			if err := f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_periods WHERE proxy_user_id=$1`, f.identity.ProxyUserID).Scan(&count); err != nil || count != 1 {
				t.Fatal("restore created a fresh zero-usage period", count, err)
			}
		})
	}
}

func TestPurchasePeriodRestoreRejectsStaleClosureAndPointer(t *testing.T) {
	for _, scenario := range []string{"new closure epoch", "new current period", "review"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			f, ctx := newPurchasePeriodRestoreFixture(t, "closing"), context.Background()
			switch scenario {
			case "new closure epoch":
				if err := f.store.ReopenXNodePeriod(ctx, f.period.ID); err != nil {
					t.Fatal(err)
				}
				current, err := f.store.XNodePeriod(ctx, f.period.ID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.store.BeginXNodePeriodClosure(ctx, current, f.now); err != nil {
					t.Fatal(err)
				}
			case "new current period":
				next := f.period
				next.ID, next.Status, next.EndedAt = "new-period", "open", nil
				if err := f.store.SaveXNodePeriod(ctx, next); err != nil {
					t.Fatal(err)
				}
			case "review":
				if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET status='review' WHERE id=$1`, f.period.ID); err != nil {
					t.Fatal(err)
				}
			}
			before, err := f.store.XNodePeriod(ctx, f.period.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, f.purchase, 0, f.now); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("stale restore accepted", err)
			}
			after, err := f.store.XNodePeriod(ctx, f.period.ID)
			if err != nil || after.Status != before.Status || after.ClosureGeneration != before.ClosureGeneration || after.UsedBytes != 77 {
				t.Fatal("stale restore changed period", after, err)
			}
		})
	}
}

func TestPurchasePeriodRestoreRechecksCurrentPurchaseAndOwner(t *testing.T) {
	for _, scenario := range []string{"same-clock quota edit", "cancelled", "due", "disabled", "purging", "settled refund"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			f, ctx := newPurchasePeriodRestoreFixture(t, "closed"), context.Background()
			at := f.now
			var err error
			switch scenario {
			case "same-clock quota edit":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_traffic_limit_bytes=2000 WHERE id=$1`, f.purchase.ID)
			case "cancelled":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE id=$1`, f.purchase.ID)
			case "due":
				at = f.purchase.ValidUntil
			case "disabled":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET access_disabled=1 WHERE proxy_user_id=$1`, f.identity.ProxyUserID)
			case "purging":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, f.identity.UserID)
			case "settled refund":
				// Retain a real refund ledger fact while simulating an old
				// implementation's later reactivation of the same paid term.
				tx, beginErr := f.store.DB().BeginTx(ctx, nil)
				if beginErr != nil {
					t.Fatal(beginErr)
				}
				defer tx.Rollback()
				balance, creditErr := adjustBalanceTx(ctx, tx, f.identity.UserID, 1, f.now)
				if creditErr != nil {
					t.Fatal(creditErr)
				}
				if _, err = insertLedgerTx(ctx, tx, f.identity.UserID, 1, balance, "admin_entitlement_refund", f.purchase.ID, "fixture", f.now); err == nil {
					err = tx.Commit()
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, f.purchase, 0, at)
			if err == nil {
				t.Fatal("ineligible restore accepted")
			}
			period, readErr := f.store.XNodePeriod(ctx, f.period.ID)
			if readErr != nil || period.Status != "closed" || period.UsedBytes != 77 || period.ClosureGeneration != f.period.ClosureGeneration {
				t.Fatal("rejected restore changed accounting", period, err, readErr)
			}
		})
	}
}
