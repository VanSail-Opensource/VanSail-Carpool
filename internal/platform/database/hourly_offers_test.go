package database

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestHourlyOffersHaveNoLegacyDefaultsAndCASCompetingEdits(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, store, 498024)
	group := xnode.Group{ID: "hourly-offer-group", Name: "Hourly offer", Enabled: true}
	if err := store.SaveXNodeGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: group.ID, Name: group.Name, BillingMode: "payg", PAYGPriceTXBMinor: 321, PriceTXBMinor: 456, Visible: true, UpstreamPresent: true})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := store.ListHourlyOffers(ctx)
	if err != nil || len(initial) != 0 {
		t.Fatal("legacy catalog seeded hourly pricing", err)
	}
	if _, err := store.HourlyOffer(ctx, group.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("absent offer became an implicit enabled product", err)
	}
	first, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID, Enabled: true, HourlyPriceTXBMinor: 123}, 0)
	if err != nil || first.Revision != 1 || first.HourlyPrice.Minor != "123" || first.CreatedAt.IsZero() || first.CreatedAt.Location().String() != "UTC" {
		t.Fatal("new offer lost exact pricing/revision/UTC timestamps", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, price := range []int64{200, 300} {
		workers.Go(func() {
			<-start
			_, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID, Enabled: true, HourlyPriceTXBMinor: price}, first.Revision)
			results <- err
		})
	}
	close(start)
	workers.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatal("offer revision did not fence competing edits", winners, conflicts)
	}
	stored, err := store.HourlyOffer(ctx, group.ID)
	if err != nil || stored.Revision != 2 || !stored.CreatedAt.Equal(first.CreatedAt) {
		t.Fatal("offer edit reset original metadata", err)
	}
	after, err := store.SquadProductByGroupID(ctx, group.ID)
	if err != nil || after.BillingMode != legacy.BillingMode || after.PAYGPriceTXBMinor != legacy.PAYGPriceTXBMinor || after.PriceTXBMinor != legacy.PriceTXBMinor {
		t.Fatal("hourly offer rewrote grandfathered catalog pricing", err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='xnode.hourly_offer.update' AND target_id=$1`, group.ID).Scan(&audits); err != nil || audits != 2 {
		t.Fatal("offer writes and audits were not atomic", audits, err)
	}
}

func TestHourlyOfferPersistsPerSquadRequiredHours(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, store, 498026)
	group := xnode.Group{ID: "hourly-required-hours", Name: "Required hours", Enabled: true}
	if err := store.SaveXNodeGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	offer, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID, Enabled: true,
		HourlyPriceTXBMinor: 100, RequiredHours: 3}, 0)
	if err != nil || offer.RequiredHours != 3 {
		t.Fatal("per-offer required hours were not stored", err, offer.RequiredHours)
	}
	read, err := store.HourlyOffer(ctx, group.ID)
	if err != nil || read.RequiredHours != 3 {
		t.Fatal("per-offer required hours were not read back", err, read.RequiredHours)
	}
	updated, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID, Enabled: true,
		HourlyPriceTXBMinor: 100}, offer.Revision)
	if err != nil || updated.RequiredHours != 0 {
		t.Fatal("clearing the override to inherit the policy failed", err, updated.RequiredHours)
	}
	if _, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID, Enabled: true,
		HourlyPriceTXBMinor: 100, RequiredHours: model.MaxHourlyRequiredHours + 1}, updated.Revision); !errors.Is(err, ErrConflict) {
		t.Fatal("out-of-range required hours reached the database", err)
	}
}

func TestHourlyOfferRejectsMissingGroupsAndUnavailablePrices(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, store, 498025)
	if _, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: "missing", Enabled: true, HourlyPriceTXBMinor: 1}, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal("offer admitted a missing native squad", err)
	}
	group := xnode.Group{ID: "disabled-hourly-offer", Name: "Disabled offer", Enabled: true}
	if err := store.SaveXNodeGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID, Enabled: true}, 0); !errors.Is(err, ErrConflict) {
		t.Fatal("zero-priced offer became purchasable", err)
	}
	offer, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID}, 0)
	if err != nil || offer.Enabled || offer.HourlyPriceTXBMinor != 0 {
		t.Fatal("explicit disabled offer required invented pricing", err)
	}
	if _, err := store.DB().ExecContext(ctx, `DELETE FROM xnode_groups WHERE id=$1`, group.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.HourlyOffer(ctx, group.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("removed native squad retained purchasable hourly offer", err)
	}
}
