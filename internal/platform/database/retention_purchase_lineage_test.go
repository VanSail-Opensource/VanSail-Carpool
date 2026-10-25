package database

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCompactionProtectsTransitiveRetainedRenewalLineage(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprintf("batch=%t", batch), func(t *testing.T) {
			s, identity, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			old := now.Add(-60 * 24 * time.Hour)
			if _, err := s.AdjustBalance(ctx, identity.UserID, 1000, "seed", "lineage-fixture", old); err != nil {
				t.Fatal(err)
			}
			combo, err := s.SaveCombo(ctx, ComboInput{Name: "Lineage", ValidityDays: 30, PriceTXBMinor: 100, TrafficLimitBytes: 1000, ResetStrategy: "DAY", Active: true})
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for index := range 4 {
				purchase, err := s.CreatePurchase(ctx, PurchaseInput{UserID: identity.UserID, ComboID: combo.ID, IdempotencyKey: fmt.Sprint(index)}, old)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=? WHERE id=?`, stamp(old), purchase.ID); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, purchase.ID)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET auto_renew_source_purchase_id=? WHERE id=?`, ids[0], ids[1]); err != nil {
				t.Fatal(err)
			}
			if batch {
				if _, err := s.db.ExecContext(ctx, `INSERT INTO renewal_batches(id,user_id,source_purchase_id,idempotency_key,request_fingerprint,term_count,charged_txb_minor,created_at) VALUES('lineage',?,?,'batch','fingerprint',1,100,?)`, identity.UserID, ids[1], stamp(old)); err != nil {
					t.Fatal(err)
				}
				if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET renewal_batch_id='lineage',renewal_index=1 WHERE id=?`, ids[2]); err != nil {
					t.Fatal(err)
				}
			} else if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET auto_renew_source_purchase_id=? WHERE id=?`, ids[1], ids[2]); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE xnode_periods SET purchase_id=?,status='closing',used_bytes=77 WHERE id=?`, ids[2], period.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET status='done'`); err != nil {
				t.Fatal(err)
			}
			for iteration := range 2 {
				tx, err := s.db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				counts := map[string]int64{}
				if err := compactPurchasesTx(ctx, tx, now.Add(-7*24*time.Hour), now, counts); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				if counts["purchases"] != int64(1-iteration) {
					t.Fatal("unreferenced history did not compact exactly once", counts)
				}
				for _, id := range ids[:3] {
					if _, err := s.PurchaseByID(ctx, id); err != nil {
						t.Fatal("retained successor lost its predecessor", id, err)
					}
				}
				var charged int64
				if err := s.db.QueryRowContext(ctx, `SELECT charged_txb_minor FROM purchase_history_tombstones WHERE purchase_id=?`, ids[3]).Scan(&charged); err != nil || charged != 100 {
					t.Fatal("compacted financial evidence changed", charged, err)
				}
			}
		})
	}
}

func TestMaintenanceRetainsAbandonedUserReferencedByAudit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, store, 87101)
	now := time.Now().UTC()
	if _, err := store.db.ExecContext(ctx, `UPDATE users SET onboarding_state='intro',created_at=? WHERE id=?`, stamp(now.Add(-48*time.Hour)), user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendAudit(ctx, &user.ID, "retention", "user", user.ID, `{}`, now); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal("referenced actor broke maintenance", err)
		}
		if _, err := store.UserByID(ctx, user.ID); err != nil {
			t.Fatal("audit attribution lost", err)
		}
	}
}
