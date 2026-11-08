package database

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestRuntimePAYGReadsFollowCurrentWalletAndCredit(t *testing.T) {
	s, parent, _, account := paygBillingFixture(t)
	ctx := context.Background()
	assertCandidates := func(want int) {
		t.Helper()
		items, err := s.XNodePAYGFundedAccounts(ctx, account.NodeID)
		if err != nil || len(items) != want {
			t.Fatal("funding candidates", items, err)
		}
		funding, err := s.XNodePAYGFunding(ctx, account.NodeID)
		if err != nil || len(funding) != len(items) {
			t.Fatal("funding projection", funding, err)
		}
		for i, item := range items {
			if !reflect.DeepEqual(funding[i], item.Funding()) {
				t.Fatal("funding values diverged from exact account", funding[i], item)
			}
		}
	}
	assertCandidates(1)
	if _, err := s.AdjustBalance(ctx, parent.UserID, -1000, "empty-runtime-wallet", "runtime", time.Now()); err != nil {
		t.Fatal(err)
	}
	assertCandidates(0)
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=10,credit_started_at=$1 WHERE proxy_user_id=$2`, stamp(time.Now().UTC()), account.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	assertCandidates(1)
	if _, err := s.AdjustBalance(ctx, parent.UserID, -1, "runtime-wallet-debt", "runtime", time.Now()); err != nil {
		t.Fatal(err)
	}
	assertCandidates(0)
	ids := make([]int64, 130)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	ids[129] = account.ProxyUserID
	items, err := s.XNodePAYGConnectedAccounts(ctx, account.NodeID, ids)
	if err != nil || len(items) != 1 || items[0].ProxyUserID != account.ProxyUserID {
		t.Fatal("chunked connected accounts", items, err)
	}
	for _, node := range []string{account.NodeID, "other-node"} {
		items, err = s.XNodePAYGConnectedAccounts(ctx, node, nil)
		if err != nil || len(items) != 0 {
			t.Fatal("empty connection inventory", items, err)
		}
	}
	items, err = s.XNodePAYGConnectedAccounts(ctx, "other-node", []int64{account.ProxyUserID})
	if err != nil || len(items) != 0 {
		t.Fatal("cross-node connection read", items, err)
	}
}

func TestPAYGFundingProjectionKeepsExactExpiryAndAccounting(t *testing.T) {
	s, parent, _, account := paygBillingFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.AdjustBalance(ctx, parent.UserID, -1000, "funding-wallet", "runtime", now); err != nil {
		t.Fatal(err)
	}
	started := now.Add(-xnode.PAYGWindow + time.Nanosecond)
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_payg_accounts SET purchased_bytes=81,credit_bytes=1,credit_started_at=$1 WHERE proxy_user_id=$2`, stamp(started), account.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=42 WHERE id=$1`, account.PeriodID); err != nil {
		t.Fatal(err)
	}
	items, err := s.XNodePAYGFunding(ctx, account.NodeID)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	funding := items[0]
	if !funding.Funded(now) || funding.Funded(now.Add(time.Nanosecond)) || !funding.CreditStartedAt.Equal(started) {
		t.Fatal("funding lost exact prepaid expiry", funding)
	}
	full, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
	if err != nil || full.UsedBytes != 42 || full.PurchasedBytes != 81 || !reflect.DeepEqual(full.Funding(), funding) {
		t.Fatal("funding disturbed exact accounting", full, funding, err)
	}
	items, err = s.XNodePAYGFunding(ctx, "other-node")
	if err != nil || len(items) != 0 {
		t.Fatal("funding leaked another node", items, err)
	}
	if _, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: account.GroupID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 200, PAYGState: "unavailable"}); err != nil {
		t.Fatal(err)
	}
	items, err = s.XNodePAYGFunding(ctx, account.NodeID)
	if err != nil || len(items) != 1 || !items[0].Disabled || items[0].UnitPriceMinor != 200 {
		t.Fatal("funding retained old price or availability", items, err)
	}
}

func TestCadenceInventoryRetainsRecoveryAndExcludesPAYG(t *testing.T) {
	s, parent, period, _ := paygBillingFixture(t)
	ctx := context.Background()
	for _, step := range []struct {
		strategy, status string
		want             int
	}{
		{"NO_RESET", "open", 0}, {"DAY", "open", 1}, {"DAY", "closing", 1},
		{"DAY", "closed", 1}, {"DAY", "review", 0},
	} {
		period.Strategy, period.Status = step.strategy, step.status
		if err := s.SaveXNodePeriod(ctx, period); err != nil {
			t.Fatal(err)
		}
		items, err := s.XNodeCadenceIdentities(ctx)
		if err != nil || len(items) != step.want || (len(items) == 1 && items[0].ProxyUserID != parent.ProxyUserID) {
			t.Fatal(step, items, err)
		}
	}
}
