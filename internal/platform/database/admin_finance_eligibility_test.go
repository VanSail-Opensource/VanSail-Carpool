package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestFinanceAndSettingsRecheckCurrentAdministrator(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 14, 0, 0, 0, time.UTC)
	actor := createTestAdmin(t, store, 89104)
	combo := saveTestCombo(t, store, "Administrator fence", 100, 30)
	member, purchase := createAdminWorkflowPurchase(t, store, 89105, combo, now)
	beforeBalance := adminWorkflowBalance(t, store, member.ID)
	config := affiliates.ConfigInput{ExpectedVersion: 1, Tiers: []affiliates.Tier{{
		Name: "Starter", Threshold: 0, Enabled: true, Reward: affiliates.Reward{Kind: "none"},
	}}}
	ipConfig, quota := iplookup.DefaultConfig(), 39
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"missing", "admin", "active", 0},
		{"demoted", "user", "active", 0},
		{"purging", "admin", "purging", 0},
		{"blocked", "admin", "active", 1},
	} {
		t.Run(state.name, func(t *testing.T) {
			// actor remains the previously authenticated administrator snapshot.
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`,
				state.role, state.status, state.blocked, actor.ID); err != nil {
				t.Fatal(err)
			}
			actorID := actor.ID
			if state.name == "missing" {
				actorID = "missing-administrator"
			}
			if _, err := store.AdjustAdminBalance(ctx, actorID, member.ID, 25, "fenced-credit", "credit", now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible credit = %v", err)
			}
			if _, err := store.DeductAdminBalance(ctx, actorID, member.ID, 50, "fenced-deduction", "deduction", now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible deduction = %v", err)
			}
			if _, err := store.CancelAdminPurchase(ctx, actorID, purchase.ID, "cancel", now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible cancellation = %v", err)
			}
			if _, err := store.SaveAffiliateConfig(ctx, actorID, config, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible affiliate settings = %v", err)
			}
			if err := store.SaveIPLookupSettings(ctx, actorID, ipConfig, map[string]string{"ipapi": "sealed"},
				map[string]*int{combo.ID: &quota}); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible IP settings = %v", err)
			}
		})
	}
	retained, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || retained.Status != purchase.Status || adminWorkflowBalance(t, store, member.ID) != beforeBalance {
		t.Fatalf("rejected administrator changed financial state: %+v, %v", retained, err)
	}
	current, err := store.AffiliateConfig(ctx)
	if err != nil || current.Version != config.ExpectedVersion {
		t.Fatalf("rejected affiliate update = %+v, %v", current, err)
	}
	var retainedQuota sql.NullInt64
	if err := store.DB().QueryRowContext(ctx, `SELECT ip_lookup_quota FROM combos WHERE id=$1`, combo.ID).Scan(&retainedQuota); err != nil || retainedQuota.Valid {
		t.Fatalf("rejected quota update = %+v, %v", retainedQuota, err)
	}
	var settings, audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM settings WHERE key IN ($1,$2)`,
		iplookup.SettingKey, iplookup.CredentialKey("ipapi")).Scan(&settings); err != nil || settings != 0 {
		t.Fatalf("rejected settings writes = %d, %v", settings, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1`, actor.ID).Scan(&audits); err != nil || audits != 0 {
		t.Fatalf("rejected success audits = %d, %v", audits, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	entry, err := store.AdjustAdminBalance(ctx, actor.ID, member.ID, 25, "fenced-credit", "credit", now)
	if err != nil || entry.BalanceAfterRaw != beforeBalance+25 {
		t.Fatalf("restored credit = %+v, %v", entry, err)
	}
	if replay, err := store.AdjustAdminBalance(ctx, actor.ID, member.ID, 25, "fenced-credit", "credit", now); err != nil || replay.ID != entry.ID {
		t.Fatalf("credit replay = %+v, %v", replay, err)
	}
	if _, err := store.DeductAdminBalance(ctx, actor.ID, member.ID, 50, "fenced-deduction", "deduction", now); err != nil {
		t.Fatal(err)
	}
	if cancelled, err := store.CancelAdminPurchase(ctx, actor.ID, purchase.ID, "cancel", now); err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("restored cancellation = %+v, %v", cancelled, err)
	}
	if saved, err := store.SaveAffiliateConfig(ctx, actor.ID, config, now); err != nil || saved.Version != 2 || len(saved.Tiers) != 1 {
		t.Fatalf("restored affiliate update = %+v, %v", saved, err)
	}
	if err := store.SaveIPLookupSettings(ctx, actor.ID, ipConfig, map[string]string{"ipapi": "sealed"}, map[string]*int{combo.ID: &quota}); err != nil {
		t.Fatal(err)
	}
	if balance := adminWorkflowBalance(t, store, member.ID); balance != beforeBalance+25-50+purchase.PriceTXBMinor {
		t.Fatalf("restored balance = %d", balance)
	}
}
