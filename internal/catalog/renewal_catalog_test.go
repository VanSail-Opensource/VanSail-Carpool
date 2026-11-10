package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestAutomaticRenewalRetainsRepricedHiddenSquads(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		present    bool
		balance    int64
		wantPrice  int64
		wantReason string
	}{
		{name: "hidden but live", present: true, balance: 1_700, wantPrice: 1_700},
		{name: "removed upstream", balance: 1_700, wantPrice: 1_000},
		{name: "low balance is checked at settlement", present: true, balance: 1_300, wantPrice: 1_700},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			plan := eligibleAutoRenewalPlan()
			plan.Purchase.PriceTXBMinor = 1_300
			plan.Addons = []model.SquadProduct{{ID: "retained", GroupID: "retained", PriceTXBMinor: 700}}
			plan.GrossMinor, plan.NetMinor = 1_700, 1_700
			repository := dueAutoRenewalRepositoryForPlan(plan)
			repository.balance = model.TXBMoney(test.balance)
			repository.addons = []model.SquadProduct{{ID: "retained", GroupID: "retained", Visible: false}}
			remote := renewalTestRemote()
			if test.present {
				remote.squads = append(remote.squads, RemoteSquad{UUID: "retained", Name: "Retained"})
				remote.accessible["retained"] = []string{"node-1"}
			}
			service := newCatalogServiceForTest(repository, remote)
			user := model.User{ID: plan.Purchase.UserID}
			status, err := service.AutomaticRenewal(ctx, user, plan.Purchase.ID)
			if err != nil || status.NetPrice.MinorInt64() != test.wantPrice || status.CanEnable != (test.wantReason == "") {
				t.Fatalf("AutomaticRenewal() = (%+v, %v)", status, err)
			}
			if test.wantReason != "" && (status.IneligibleReason == nil || *status.IneligibleReason != test.wantReason) {
				t.Fatalf("renewal reason = %v, want %s", status.IneligibleReason, test.wantReason)
			}
			updated, err := service.SetAutomaticRenewal(ctx, user, plan.Purchase.ID, true)
			if test.wantReason == "" {
				if err != nil || !updated.Enabled || !repository.setEnabled || updated.NetPrice != status.NetPrice {
					t.Fatalf("SetAutomaticRenewal() = (%+v, %v)", updated, err)
				}
			} else if !errors.Is(err, ErrAutoRenewalIneligible) || repository.setEnabled {
				t.Fatalf("SetAutomaticRenewal(ineligible) = %v, enabled=%t", err, repository.setEnabled)
			}
			if err := service.ProcessDueAutoRenewals(ctx, service.now()); err != nil {
				t.Fatal(err)
			}
			if test.wantReason == "" {
				if len(repository.commitIDs) != 1 || len(repository.failed) != 0 {
					t.Fatalf("renewal commits=%v failures=%v", repository.commitIDs, repository.failed)
				}
			} else if len(repository.commitIDs) != 0 || len(repository.failed) != 1 || repository.failed[0].reason != test.wantReason {
				t.Fatalf("blocked renewal commits=%v failures=%v", repository.commitIDs, repository.failed)
			}
			public, err := service.Catalog(ctx)
			if err != nil || len(public.Addons) != 0 {
				t.Fatalf("public catalog exposed hidden add-on: (%+v, %v)", public, err)
			}
		})
	}
}

// A later billing-mode edit is merchandising, not ownership: both an included
// squad and a purchased add-on stay in the owned renewal selection after their
// product rows report PAYG.
func TestRenewalRetainsOwnedSquadsAfterBillingModeEdits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := renewalTestRepository()
	repository.quote.AddonSquadUUIDs = []string{"retained"}
	repository.addons = []model.SquadProduct{
		{GroupID: "core-squad", BillingMode: "payg"},
		{GroupID: "retained", BillingMode: "payg", Visible: false},
	}
	remote := renewalTestRemote()
	remote.squads = append(remote.squads, RemoteSquad{UUID: "retained", Name: "Retained"})
	remote.accessible["retained"] = []string{"node-1"}
	service := newCatalogServiceForTest(repository, remote)
	user := model.User{ID: "user", OnboardingState: "complete"}
	quote, err := service.RenewalQuote(ctx, user, "purchase", 1)
	if err != nil || len(quote.AddonSquadUUIDs) != 1 || quote.AddonSquadUUIDs[0] != "retained" || len(quote.AccessibleNodes) != 1 {
		t.Fatalf("RenewalQuote(owned PAYG edits) = (%+v, %v)", quote, err)
	}
}

// New-sale visibility is unchanged: an owned renewal keeps PAYG-marked products,
// but the storefront still hides them as standalone add-ons.
func TestStorefrontStillExcludesPAYGAddons(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := renewalTestRepository()
	repository.addons = []model.SquadProduct{{GroupID: "payg-addon", Name: "Metered", Visible: true, BillingMode: "payg"}}
	remote := renewalTestRemote()
	remote.squads = append(remote.squads, RemoteSquad{UUID: "payg-addon", Name: "Metered"})
	public, err := newCatalogServiceForTest(repository, remote).Catalog(ctx)
	if err != nil || len(public.Addons) != 0 {
		t.Fatalf("storefront exposed PAYG add-on: (%+v, %v)", public, err)
	}
}

func TestRenewalNodesIncludeUnlistedOwnedSquads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := renewalTestRepository()
	repository.quote.AddonSquadUUIDs = []string{"retained"}
	remote := renewalTestRemote()
	remote.squads = append(remote.squads, RemoteSquad{UUID: "retained", Name: "Retained"})
	remote.accessible = map[string][]string{"retained": {"node-1"}}
	service := newCatalogServiceForTest(repository, remote)
	user := model.User{ID: "user", OnboardingState: "complete"}
	quote, err := service.RenewalQuote(ctx, user, "purchase", 1)
	if err != nil || len(quote.AccessibleNodes) != 1 {
		t.Fatalf("RenewalQuote(owned, unlisted) = (%+v, %v)", quote, err)
	}
	remote.squads = remote.squads[:1]
	remote.accessible["core-squad"] = []string{"node-1"}
	quote, err = service.RenewalQuote(ctx, user, "purchase", 1)
	if err != nil || len(quote.AddonSquadUUIDs) != 0 || len(quote.AccessibleNodes) != 1 {
		t.Fatalf("RenewalQuote(missing add-on) = (%+v, %v), want the core-only renewal", quote, err)
	}
}

// TestOwnedRenewalHydratesHiddenCombo: a retired (inactive) combo row remains
// renewable through both owned entry points, while the member storefront keeps
// the active-only read.
func TestOwnedRenewalHydratesHiddenCombo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := renewalTestRepository()
	repository.combo.Active = false
	repository.catalogRepository.combos = nil
	service := newCatalogServiceForTest(repository, renewalTestRemote())
	user := model.User{ID: "user", OnboardingState: "complete"}
	quote, err := service.RenewalQuote(ctx, user, "purchase", 1)
	if err != nil || len(quote.AccessibleNodes) != 1 {
		t.Fatalf("RenewalQuote(hidden combo) = (%+v, %v)", quote, err)
	}
	repository.renewal = model.RenewalBatch{ID: "batch", PurchaseID: "purchase", TermCount: 1}
	if batch, err := service.Renew(ctx, user, "purchase", 1, "hidden-combo"); err != nil || batch.ID != "batch" {
		t.Fatalf("Renew(hidden combo) = (%+v, %v)", batch, err)
	}
	public, err := service.Catalog(ctx)
	if err != nil || len(public.Combos) != 0 || !repository.catalogRepository.combosVisibleOnly {
		t.Fatalf("storefront = (%+v, %v), visibleOnly=%t", public, err, repository.catalogRepository.combosVisibleOnly)
	}
}

// TestOwnedRenewalFailsClosedWithoutComboRow: a purchase whose combo cannot be
// resolved must not renew against a fallback.
func TestOwnedRenewalFailsClosedWithoutComboRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := renewalTestRepository()
	repository.comboErr = database.ErrNotFound
	service := newCatalogServiceForTest(repository, renewalTestRemote())
	user := model.User{ID: "user", OnboardingState: "complete"}
	if _, err := service.RenewalQuote(ctx, user, "purchase", 1); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("RenewalQuote(missing combo) = %v, want ErrNotFound", err)
	}
}

// TestAutomaticRenewalKeepsHiddenComboEligible: an inactive combo must not
// become a terminal combo_unavailable failure during due settlement.
func TestAutomaticRenewalKeepsHiddenComboEligible(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	plan := eligibleAutoRenewalPlan()
	plan.Combo.Active = false
	repository := dueAutoRenewalRepositoryForPlan(plan)
	service := newCatalogServiceForTest(repository, renewalTestRemote())
	user := model.User{ID: "user"}
	status, err := service.AutomaticRenewal(ctx, user, plan.Purchase.ID)
	if err != nil || !status.CanEnable || status.IneligibleReason != nil {
		t.Fatalf("AutomaticRenewal(hidden combo) = (%+v, %v)", status, err)
	}
	if err := service.ProcessDueAutoRenewals(ctx, service.now()); err != nil || len(repository.commitIDs) != 1 || len(repository.failed) != 0 {
		t.Fatalf("ProcessDueAutoRenewals = %v, commits=%v failures=%v", err, repository.commitIDs, repository.failed)
	}
}
