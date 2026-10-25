package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"sync"
	"testing"
	"time"
)

func TestPAYGNodesSpendOneWalletWithoutDuplicatingCredit(t *testing.T) {
	s, parent, _, first := paygBillingFixture(t)
	ctx := context.Background()
	if _, err := s.AdjustBalance(ctx, parent.UserID, -900, "one-gb", "seed", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "node-two", Name: "Second PAYG", ConfigID: "config", Enabled: true, MultiplierFP: 10_000_000, SettingsCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	i, err := s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "member-payg-second", CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	second := xnode.PAYGAccount{ProxyUserID: i.ProxyUserID, UserID: parent.UserID, GroupID: first.GroupID, NodeID: "node-two", PeriodID: "payg-second", UnitPriceMinor: 100}
	if err = s.SaveXNodePeriod(ctx, xnode.Period{ID: second.PeriodID, ProxyUserID: i.ProxyUserID, StartedAt: time.Now(), Strategy: "PAYG", Status: "open"}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveXNodePAYGAccount(ctx, second); err != nil {
		t.Fatal(err)
	}
	// Both nodes have already consumed their first prepaid GB. Continuation
	// observations can purchase only one further GB from this wallet.
	for _, a := range []xnode.PAYGAccount{first, second} {
		if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=? WHERE id=?`, xnode.PAYGGB, a.PeriodID); err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_payg_accounts SET purchased_bytes=?,credit_started_at=? WHERE proxy_user_id=?`, xnode.PAYGGB, stamp(time.Now()), a.ProxyUserID); err != nil {
			t.Fatal(err)
		}
		// Outstanding metered traffic, rather than an idle exhausted allowance,
		// is required before an observation may buy a replacement GB.
		if _, err = s.DB().ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through,traffic_bytes,unbilled_bytes) VALUES(?,?,?,1)`, a.ProxyUserID, paygWindowStamp(time.Now().Add(-time.Second)), xnode.PAYGStartBytes); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for _, a := range []xnode.PAYGAccount{first, first, second, second} {
		wg.Add(1)
		go func(a xnode.PAYGAccount) {
			defer wg.Done()
			errs <- s.ObserveXNodePAYGConnections(ctx, a.NodeID, []int64{a.ProxyUserID}, time.Now())
		}(a)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	one, _ := s.XNodePAYGAccount(ctx, first.ProxyUserID)
	two, _ := s.XNodePAYGAccount(ctx, second.ProxyUserID)
	if one.PurchasedBytes+two.PurchasedBytes != 3*xnode.PAYGGB || one.BalanceMinor != 0 || two.BalanceMinor != 0 {
		t.Fatal("wallet race or cross-node sharing", one, two)
	}
}

func TestPAYGMinuteWindowAndRetention(t *testing.T) {
	s, parent, _, a := paygBillingFixture(t)
	ctx := context.Background()
	asOf := time.Now().UTC().Truncate(time.Minute)
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{asOf.Add(-72*time.Hour + time.Minute), asOf.Add(-time.Minute), asOf} {
		if err = recordPAYGMinuteTx(ctx, tx, parent.UserID, a.GroupID, at, 17, 3); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	usage, err := s.XNodePAYGUsage(ctx, parent.UserID, asOf.Add(-72*time.Hour), asOf)
	if err != nil || len(usage) != 1 || usage[0].TrafficBytes != 34 || usage[0].SpentMinor != 6 {
		t.Fatal("window/current partial boundary", usage, err)
	}
	if _, err = s.DB().ExecContext(ctx, `INSERT INTO xnode_payg_minutes(user_id,group_id,minute,traffic_bytes) VALUES(?,?,?,1)`, parent.UserID, a.GroupID, stamp(asOf.Add(-74*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PrunePAYGTransientRecords(ctx, asOf); err != nil {
		t.Fatal(err)
	}
	var old int
	if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_payg_minutes WHERE minute<?`, stamp(asOf.Add(-73*time.Hour))).Scan(&old); err != nil || old != 0 {
		t.Fatal("expired aggregates retained", old, err)
	}
}
