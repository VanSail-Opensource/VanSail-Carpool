package database

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

const customComboRecoveryMigration = "054_restore_custom_combo_after_core_reward.sql"

func TestCustomComboRecoveryRequiresUnambiguousRewardEvidence(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC().Add(-2 * time.Hour)
	core := saveTestCombo(t, store, "Original core", 100, 30)
	target := saveTestCombo(t, store, "New core", 200, 45)
	cases := []struct {
		name                         string
		recover                      bool
		userID, purchaseID, resultID string
		ledgerCount                  int
		price, balance               int64
	}{
		{name: "erased custom terms", recover: true, price: 54000},
		{name: "same settlement timestamp", recover: true, price: 54000},
		{name: "zero price and existing sync", recover: true},
		{name: "pending rollover metadata", recover: true, price: 54000},
		{name: "no core reward", price: 54000},
		{name: "ambiguous custom prizes", price: 54000},
		{name: "traffic reward", price: 54000},
		{name: "administrator edit", price: 54000},
		{name: "expired term", price: 54000},
		{name: "partial overrides", price: 54000},
		{name: "different current core", price: 54000},
		{name: "rollover in progress", price: 54000},
		{name: "malformed historical reward", price: 54000},
		{name: "later paid squad addition", price: 54000},
	}
	for index := range cases {
		test := &cases[index]
		user, purchase := createAdminWorkflowPurchase(t, store, int64(31910+index), core, now)
		test.userID, test.purchaseID = user.ID, purchase.ID
		if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, false, now); err != nil {
			t.Fatal(err)
		}
		reward := activity.Reward{Kind: activity.RewardEntitlementGrant, ComboID: core.ID, SquadUUIDs: []string{"awarded"},
			RenewalPriceMinor: test.price, TrafficLimitBytes: 500 << 30, RolloverMinRemainingBPS: 9999}
		grant := playComboReward(t, store, user.ID, "custom", reward, now.Add(time.Minute))
		test.resultID = grant.ID
		if test.name == "ambiguous custom prizes" {
			playComboReward(t, store, user.ID, "second custom", reward, now.Add(time.Minute))
		}
		if test.name == "traffic reward" {
			playComboReward(t, store, user.ID, "traffic", activity.Reward{Kind: activity.RewardTrafficGrant,
				Range: &activity.ValueRange{Min: 10, Max: 10, Distribution: "uniform"}}, now.Add(90*time.Second))
		}
		changeAt := now.Add(2 * time.Minute)
		if test.name == "same settlement timestamp" {
			changeAt = grant.CreatedAt
		}
		if test.name != "no core reward" {
			playComboReward(t, store, user.ID, "core", activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}, changeAt)
		}
		// Reproduce the exact destructive write performed by the old handler.
		if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET combo_id=?,entitlement_squad_uuids=NULL,
			entitlement_addon_squad_uuids=NULL,entitlement_traffic_limit_bytes=NULL,reward_renewal_price_minor=NULL,
			reward_rollover_min_remaining_bps=NULL,reward_renewal_traffic_limit_bytes=NULL,reward_traffic_renewal=1 WHERE id=?`, target.ID, purchase.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind='remna_sync_user' AND payload=?`, fmt.Sprintf(`{"userId":%q}`, user.ID)); err != nil {
			t.Fatal(err)
		}
		switch test.name {
		case "pending rollover metadata":
			seedPendingRewardRollover(t, store, purchase.ID, changeAt)
		case "zero price and existing sync":
			if _, err := store.DB().ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,available_at,created_at,updated_at)
				VALUES(?,'remna_sync_user',?,'pending',?,?,?)`, "existing-sync", fmt.Sprintf(`{"userId":%q}`, user.ID), stamp(now), stamp(now), stamp(now)); err != nil {
				t.Fatal(err)
			}
		case "administrator edit":
			if err := store.AppendAudit(ctx, &user.ID, "entitlement.combo_replace", "purchase", purchase.ID, "{}", changeAt); err != nil {
				t.Fatal(err)
			}
		case "expired term":
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired' WHERE id=?`, purchase.ID); err != nil {
				t.Fatal(err)
			}
		case "partial overrides":
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_squad_uuids='["edited"]' WHERE id=?`, purchase.ID); err != nil {
				t.Fatal(err)
			}
		case "different current core":
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET combo_id=? WHERE id=?`, core.ID, purchase.ID); err != nil {
				t.Fatal(err)
			}
		case "rollover in progress":
			seedPendingRewardRollover(t, store, purchase.ID, changeAt)
			if err := store.MarkRolloverProcessing(ctx, purchase.ID, changeAt); err != nil {
				t.Fatal(err)
			}
		case "malformed historical reward":
			if _, err := store.DB().ExecContext(ctx, `UPDATE activity_draw_results SET reward_payload='invalid' WHERE id=?`, grant.ID); err != nil {
				t.Fatal(err)
			}
		case "later paid squad addition":
			if _, err := store.DB().ExecContext(ctx, `INSERT INTO purchase_addon_adjustments(id,purchase_id,idempotency_key,
				request_fingerprint,charged_txb_minor,created_at) VALUES(?,?,'addon','recovery-fixture-fingerprint',0,?)`,
				"later-addon", purchase.ID, stamp(changeAt)); err != nil {
				t.Fatal(err)
			}
		}
		test.ledgerCount = adminWorkflowLedgerCount(t, store, user.ID, "activity_draw_fee")
		test.balance = adminWorkflowBalance(t, store, user.ID)
	}
	for range 2 {
		if _, err := store.DB().ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=?`, customComboRecoveryMigration); err != nil {
			t.Fatal(err)
		}
		if err := migrate(ctx, store.DB()); err != nil {
			t.Fatal(err)
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				var price, traffic, threshold sql.NullInt64
				var squads sql.NullString
				var enabled, auditCount, jobs int
				if err := store.DB().QueryRowContext(ctx, `SELECT reward_renewal_price_minor,entitlement_traffic_limit_bytes,
					reward_rollover_min_remaining_bps,entitlement_squad_uuids,auto_renew_enabled FROM purchases WHERE id=?`, test.purchaseID).
					Scan(&price, &traffic, &threshold, &squads, &enabled); err != nil {
					t.Fatal(err)
				}
				if price.Valid != test.recover || enabled != 0 {
					t.Fatalf("recovered=%t enabled=%d, want recovered=%t disabled", price.Valid, enabled, test.recover)
				}
				if test.recover && (price.Int64 != test.price || traffic.Int64 != 500<<30 || threshold.Int64 != 9999 || squads.String != `["awarded"]`) {
					t.Fatalf("recovered terms = (%+v, %+v, %+v, %+v)", price, traffic, threshold, squads)
				}
				if test.name == "partial overrides" && squads.String != `["edited"]` {
					t.Fatal("recovery overwrote newer squad overrides")
				}
				if adminWorkflowBalance(t, store, test.userID) != test.balance {
					t.Fatal("recovery changed the member balance")
				}
				if test.name == "pending rollover metadata" || test.name == "rollover in progress" {
					var state string
					var limit, minimum int64
					if err := store.DB().QueryRowContext(ctx, `SELECT status,traffic_limit_bytes,minimum_remaining_bps FROM purchase_rollovers WHERE purchase_id=?`, test.purchaseID).Scan(&state, &limit, &minimum); err != nil {
						t.Fatal(err)
					}
					if test.recover && (state != "pending" || limit != 500<<30 || minimum != 9999) {
						t.Fatalf("recovered pending rollover = (%s, %d, %d)", state, limit, minimum)
					}
					if !test.recover && (state != "processing" || limit != target.TrafficLimitBytes || minimum != int64(target.RolloverMinRemainingBPS)) {
						t.Fatalf("processing rollover was modified = (%s, %d, %d)", state, limit, minimum)
					}
				}
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE target_id=? AND action='activity.custom_combo_recovered'`, test.purchaseID).Scan(&auditCount); err != nil {
					t.Fatal(err)
				}
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='remna_sync_user' AND payload=?`, fmt.Sprintf(`{"userId":%q}`, test.userID)).Scan(&jobs); err != nil {
					t.Fatal(err)
				}
				if auditCount != boolInt(test.recover) || jobs != boolInt(test.recover) || adminWorkflowLedgerCount(t, store, test.userID, "activity_draw_fee") != test.ledgerCount {
					t.Fatalf("recovery side effects: audits=%d jobs=%d", auditCount, jobs)
				}
			})
		}
	}
}
