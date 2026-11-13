package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGNodeCostsUseOwnerChargesAndWindow(t *testing.T) {
	s, owner, combo, account := paygBillingFixture(t)
	ctx := context.Background()
	before := time.Now().UTC().Add(-time.Minute)
	paygReport(t, s, account, 1, uint64(xnode.PAYGStartBytes))
	after := time.Now().UTC().Add(time.Minute)
	costs, truncated, err := s.XNodePAYGNodeCosts(ctx, owner.UserID, before, after)
	if err != nil || truncated || len(costs) != 1 || costs[0].NodeID != account.NodeID || costs[0].SpentMinor != "100" {
		t.Fatal("PAYG charges were not attributed exactly", costs, truncated, err)
	}
	if period, err := s.XNodePeriod(ctx, combo.ID); err != nil || period.UsedBytes != 0 {
		t.Fatal("PAYG history changed combo accounting", period, err)
	}
	for _, query := range []struct {
		owner    string
		from, to time.Time
	}{{"another-owner", before, after}, {owner.UserID, before.Add(-time.Hour), before}} {
		costs, _, err := s.XNodePAYGNodeCosts(ctx, query.owner, query.from, query.to)
		if err != nil || len(costs) != 0 {
			t.Fatal("another owner's or out-of-window spending leaked", costs, err)
		}
	}
	if _, _, err := s.XNodePAYGNodeCosts(ctx, owner.UserID, after, before); err != xnode.ErrInvalid {
		t.Fatal("reversed window accepted", err)
	}
}

func TestPAYGNodeCostsRetainNanosecondLedgerWindow(t *testing.T) {
	s, owner, _, account := paygBillingFixture(t)
	ctx := context.Background()
	paygReport(t, s, account, 1, uint64(xnode.PAYGStartBytes))
	at := time.Date(2030, 1, 2, 3, 4, 5, 1, time.UTC)
	if _, err := s.db.ExecContext(ctx, `UPDATE ledger_entries SET created_at=$1 WHERE id IN
 (SELECT ledger_id FROM xnode_payg_charges WHERE proxy_user_id=$2)`, stamp(at), account.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		at   time.Time
		want int
	}{{at.Add(-time.Nanosecond), 0}, {at, 1}, {at.Add(time.Nanosecond), 0}} {
		costs, truncated, err := s.XNodePAYGNodeCosts(ctx, owner.UserID, step.at, step.at)
		if err != nil || truncated || len(costs) != step.want {
			t.Fatal("ledger timestamp rounded across exact window", step, costs, truncated, err)
		}
	}
}
