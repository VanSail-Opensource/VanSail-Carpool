package database

import (
	"context"
	"testing"
	"time"
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
	}
	assertCandidates(1)
	if _, err := s.AdjustBalance(ctx, parent.UserID, -1000, "empty-runtime-wallet", "runtime", time.Now()); err != nil {
		t.Fatal(err)
	}
	assertCandidates(0)
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=10,credit_started_at=? WHERE proxy_user_id=?`, stamp(time.Now().UTC()), account.ProxyUserID); err != nil {
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
