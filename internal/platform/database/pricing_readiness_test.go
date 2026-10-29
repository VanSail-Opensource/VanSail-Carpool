package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func TestPAYGReadinessUsesPublishedValidityAcrossCalculationBoundary(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(50 * time.Second)
	actor := pricingTestAdmin(t, s)
	group, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: "ready-payg", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	p := pricing.DefaultPolicy("payg", group.GroupID, 100)
	p.DynamicEnabled, p.PurchaseMode = true, "DYNAMIC"
	p, err = s.SavePricingPolicy(ctx, actor.ID, "enable future offers", p, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.recordUnchangedProduct(ctx, "payg", group.GroupID, "purchase-prior-v1", string(pricing.ForecastStale), now); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{now, now.Add(20 * time.Second), now.Add(25 * time.Hour)} {
		items := []map[string]any{{"policy": p, "fixedPriceMinor": "100"}}
		if err = s.pricingProductReadiness(ctx, items, at); err != nil {
			t.Fatal(err)
		}
		if items[0]["currentPriceMinor"] != "100" || items[0]["pricingStatus"] != "UNCHANGED" || items[0]["pricingReason"] != string(pricing.ForecastStale) || items[0]["currentPriceValidUntil"] != pricingStamp(now.AddDate(1, 0, 0)) {
			t.Fatalf("calculation cadence expired a valid offer: %+v", items[0])
		}
	}
	items := []map[string]any{{"policy": p, "fixedPriceMinor": "100"}}
	if err = s.pricingProductReadiness(ctx, items, now.AddDate(1, 0, 1)); err != nil || items[0]["pricingStatus"] != "STALE" || items[0]["currentPriceMinor"] != nil {
		t.Fatalf("expired publication remained available: %+v %v", items, err)
	}
}
