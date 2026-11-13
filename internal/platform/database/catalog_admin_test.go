package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAdminCatalogMutationRollsBackWithAudit(t *testing.T) {
	for _, operation := range []string{"create combo", "edit combo", "hide combo", "save squad", "remove override"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			actor := createTestUser(t, store, 89101)
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
				t.Fatal(err)
			}
			combo := saveTestCombo(t, store, "Before", 100, 30)
			squad := saveTestSquad(t, store, "squad", 10, true)
			if _, err := store.AdjustBalance(ctx, actor.ID, 100, "seed", "test", time.Now()); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: actor.ID, ComboID: combo.ID, IdempotencyKey: "catalog-edit"}, time.Now()); err != nil {
				t.Fatal(err)
			}
			var beforeJobs, beforeAudits int
			if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM outbox_jobs),(SELECT COUNT(*) FROM audit_events)`).Scan(&beforeJobs, &beforeAudits); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `CREATE FUNCTION reject_catalog_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$`); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `CREATE TRIGGER reject_catalog_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.target_type IN ('combo','squad_product')) EXECUTE FUNCTION reject_catalog_audit()`); err != nil {
				t.Fatal(err)
			}
			input := ComboInput{ID: combo.ID, Name: "After", PriceTXBMinor: 200, ValidityDays: 30, TrafficLimitBytes: 1024, ResetStrategy: "MONTH_ROLLING", Active: true}
			var err error
			switch operation {
			case "create combo":
				input.ID = ""
				_, err = store.SaveAdminCombo(ctx, actor.ID, input)
			case "edit combo":
				_, err = store.SaveAdminCombo(ctx, actor.ID, input)
			case "hide combo":
				err = store.DeleteAdminCombo(ctx, actor.ID, combo.ID)
			case "save squad":
				_, err = store.SaveAdminSquadProduct(ctx, actor.ID, SquadProductInput{GroupID: squad.GroupID, UpstreamPresent: true, Description: "After", Visible: true})
			case "remove override":
				_, err = store.SaveAdminSquadProduct(ctx, actor.ID, SquadProductInput{GroupID: squad.GroupID, UpstreamPresent: true})
			}
			if err == nil {
				t.Fatal("mutation ignored unavailable audit")
			}
			storedCombo, err := store.ComboByID(ctx, combo.ID, false)
			if err != nil || storedCombo.Name != combo.Name || storedCombo.PriceTXBMinor != combo.PriceTXBMinor || !storedCombo.Active {
				t.Fatal("failed audit left a catalog edit", storedCombo, err)
			}
			storedSquad, err := store.SquadProductByGroupID(ctx, squad.GroupID)
			if err != nil || storedSquad.Description != squad.Description || storedSquad.PriceTXBMinor != squad.PriceTXBMinor || !storedSquad.Visible {
				t.Fatal("failed audit left a squad override edit/removal", storedSquad, err)
			}
			var combos, jobs, audits int
			if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM combos),(SELECT COUNT(*) FROM outbox_jobs),(SELECT COUNT(*) FROM audit_events)`).Scan(&combos, &jobs, &audits); err != nil {
				t.Fatal(err)
			}
			if combos != 1 || jobs != beforeJobs || audits != beforeAudits {
				t.Fatal("failed transaction retained a creation, dependent job or audit", combos, jobs, audits)
			}
		})
	}
}

func TestAdminCatalogRequiresCurrentActorAndRecordsCommittedChange(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89102)
	input := ComboInput{Name: "Offer", PriceTXBMinor: 100, ValidityDays: 30, TrafficLimitBytes: 1024, ResetStrategy: "MONTH_ROLLING", Active: true}
	for _, actorID := range []string{"", actor.ID} {
		if _, err := store.SaveAdminCombo(ctx, actorID, input); !errors.Is(err, ErrConflict) {
			t.Fatal("missing/non-admin actor accepted", err)
		}
		if err := store.DeleteAdminCombo(ctx, actorID, "missing"); !errors.Is(err, ErrConflict) {
			t.Fatal("hide skipped actor authorization", err)
		}
		if _, err := store.SaveAdminSquadProduct(ctx, actorID, SquadProductInput{GroupID: "squad", UpstreamPresent: true}); !errors.Is(err, ErrConflict) {
			t.Fatal("squad save skipped actor authorization", err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	combo, err := store.SaveAdminCombo(ctx, actor.ID, input)
	if err != nil || combo.Name != input.Name || combo.PriceTXBMinor != input.PriceTXBMinor {
		t.Fatal("committed combo snapshot missing", combo, err)
	}
	if err = store.DeleteAdminCombo(ctx, actor.ID, combo.ID); err != nil {
		t.Fatal(err)
	}
	product, err := store.SaveAdminSquadProduct(ctx, actor.ID, SquadProductInput{GroupID: "squad", Name: "Live name", UpstreamPresent: true, Visible: true})
	if err != nil || product.Name != "Live name" || !product.Visible {
		t.Fatal("committed squad snapshot missing", product, err)
	}
	events, err := store.ListAuditEvents(ctx, 100)
	if err != nil || len(events) != 3 {
		t.Fatal("catalog changes did not each have one audit", events, err)
	}
	for _, event := range events {
		if event.ActorUserID == nil || *event.ActorUserID != actor.ID || event.TargetID == "" {
			t.Fatal("catalog audit lost actor or target", event)
		}
	}
}

func TestCatalogSquadRetainsOmittedActivationAndNonDefaultState(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	input := SquadProductInput{GroupID: "retained", UpstreamPresent: true, PAYGState: "hidden", ActivationRequired: true, ActivationCode: "setup-code"}
	if _, err := store.SaveSquadProduct(ctx, input); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := store.DB().QueryRowContext(ctx, `SELECT activation_code_hash FROM squad_product_overrides WHERE group_id=$1`, input.GroupID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	input.PAYGState, input.ActivationCode = "", ""
	product, err := store.SaveSquadProduct(ctx, input)
	if err != nil || product.PAYGState != "hidden" || !product.ActivationRequired {
		t.Fatal("omitted fields lost prior state", product, err)
	}
	var unchanged bool
	if err := store.DB().QueryRowContext(ctx, `SELECT activation_code_hash=$1 FROM squad_product_overrides WHERE group_id=$2`, hash, input.GroupID).Scan(&unchanged); err != nil || !unchanged {
		t.Fatal("omitted activation code changed its hash", err)
	}
	input.ActivationRequired = false
	product, err = store.SaveSquadProduct(ctx, input)
	if err != nil || product.PAYGState != "hidden" {
		t.Fatal("default merchandising erased non-default availability", product, err)
	}
	input.PAYGState = "normal"
	if _, err = store.SaveSquadProduct(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SquadProductByGroupID(ctx, input.GroupID); !errors.Is(err, ErrNotFound) {
		t.Fatal("restoring all defaults retained an unnecessary row", err)
	}
}
