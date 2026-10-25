package database

import (
	"context"
	"testing"
	"time"
)

func TestPAYGUnavailableBlocksNewCreditAndPreservesFinancialHistory(t *testing.T) {
	t.Parallel()
	store, _, _, account := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if _, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: account.GroupID,
		UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100, PAYGState: "unavailable"}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Balance(ctx, account.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ObserveXNodePAYGConnections(ctx, account.NodeID, []int64{account.ProxyUserID}, now); err != nil {
		t.Fatal(err)
	}
	paused, err := store.XNodePAYGAccount(ctx, account.ProxyUserID)
	if err != nil || !paused.Disabled || paused.PurchasedBytes != account.PurchasedBytes {
		t.Fatal("unavailable squad purchased new credit", paused, err)
	}
	after, err := store.Balance(ctx, account.UserID)
	if err != nil || after.Minor != before.Minor {
		t.Fatal("unavailable admission charged wallet", before, after, err)
	}
	// An old client omitting the new field must preserve administrator intent.
	product, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: account.GroupID,
		UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100})
	if err != nil || product.PAYGState != "unavailable" {
		t.Fatal("old client silently restored availability", product, err)
	}
}
