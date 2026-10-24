package database

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

func paygBillingFixture(t *testing.T) (*Store, xnode.Identity, xnode.Period, xnode.PAYGAccount) {
	t.Helper()
	ctx := context.Background()
	s, parent, combo := xnodeTrafficFixture(t)
	if _, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: "payg-squad", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	i, err := s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "member-payg-fixture", CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	p := xnode.Period{ID: "payg-fixture", ProxyUserID: i.ProxyUserID, StartedAt: time.Now().UTC(), Strategy: "PAYG", Status: "open"}
	if err = s.SaveXNodePeriod(ctx, p); err != nil {
		t.Fatal(err)
	}
	a := xnode.PAYGAccount{ProxyUserID: i.ProxyUserID, UserID: parent.UserID, GroupID: "payg-squad", NodeID: "node", PeriodID: p.ID, UnitPriceMinor: 100}
	if err = s.SaveXNodePAYGAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AdjustBalance(ctx, parent.UserID, 1000, "payg-seed", "seed", time.Now()); err != nil {
		t.Fatal(err)
	}
	return s, parent, combo, a
}

func paygReport(t *testing.T, s *Store, a xnode.PAYGAccount, sequence uint64, bytes uint64) {
	t.Helper()
	at := time.Now().UTC()
	report := wire.TrafficReport{ID: fmt.Sprintf("agent:%d", sequence), Traffic: wire.Traffic{int(a.ProxyUserID): {bytes, 0}}, Samples: []wire.TrafficSample{{UserID: int(a.ProxyUserID), AccountingPeriod: a.PeriodID, From: at, Through: at, UploadBytes: bytes}}}
	for repeat := 0; repeat < 2; repeat++ {
		if err := s.RecordXNodeTraffic(context.Background(), a.NodeID, "agent", sequence, report.ID, report); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPAYGAutomaticGBReconnectPriceAndDebt(t *testing.T) {
	s, parent, combo, a := paygBillingFixture(t)
	ctx := context.Background()
	for repeat := 0; repeat < 2; repeat++ {
		if err := s.ObserveXNodePAYGConnections(ctx, a.NodeID, []int64{a.ProxyUserID}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	paygReport(t, s, a, 1, uint64(xnode.PAYGStartBytes-1))
	value, err := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.UsedBytes != xnode.PAYGStartBytes-1 || value.PurchasedBytes != 0 || value.BalanceMinor != 1000 {
		t.Fatal("funding/idle/sub-threshold traffic charged", value, err)
	}
	paygReport(t, s, a, 2, 1)
	value, err = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.UsedBytes != xnode.PAYGStartBytes || value.PurchasedBytes != xnode.PAYGGB || value.BalanceMinor != 900 {
		t.Fatalf("first/reconnect/raw=%+v,%v", value, err)
	}
	if period, err := s.XNodePeriod(ctx, combo.ID); err != nil || period.UsedBytes != 0 {
		t.Fatal("PAYG consumed combo quota", period, err)
	}
	if _, err = s.SaveSquadProduct(ctx, SquadProductInput{GroupID: a.GroupID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 200}); err != nil {
		t.Fatal(err)
	}
	paygReport(t, s, a, 3, uint64(xnode.PAYGGB-xnode.PAYGStartBytes+1))
	value, err = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.UsedBytes != xnode.PAYGGB+1 || value.PurchasedBytes != 2*xnode.PAYGGB || value.BalanceMinor != 700 {
		t.Fatalf("price boundary=%+v,%v", value, err)
	}
	if _, err = s.AdjustBalance(ctx, parent.UserID, -700, "payg-empty-wallet", "seed", time.Now()); err != nil {
		t.Fatal(err)
	}
	paygReport(t, s, a, 4, 1)
	value, _ = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if value.BalanceMinor != 0 || value.PurchasedBytes != 2*xnode.PAYGGB {
		t.Fatal("paid remainder charged again", value)
	}
	paygReport(t, s, a, 5, uint64(xnode.PAYGGB))
	value, _ = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if value.BalanceMinor != -200 || value.PurchasedBytes != 3*xnode.PAYGGB {
		t.Fatal("late usage was lost instead of settled", value)
	}
	var charges int
	var total int64
	if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*),SUM(blocks*unit_price_minor) FROM xnode_payg_charges WHERE proxy_user_id=?`, a.ProxyUserID).Scan(&charges, &total); err != nil || charges != 3 || total != 500 {
		t.Fatal(charges, total, err)
	}
	owner, err := s.XNodeIdentity(ctx, a.ProxyUserID)
	if err != nil || owner.MemberOwnerID() != parent.UserID || len(owner.SubscriptionHash) != 0 {
		t.Fatal("internal owner/bearer isolation", owner, err)
	}
}

func TestPAYGFailedTrafficDoesNotAdvanceCursorOrDebit(t *testing.T) {
	s, _, _, a := paygBillingFixture(t)
	ctx := context.Background()
	at := time.Now()
	report := wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{{UserID: int(a.ProxyUserID), AccountingPeriod: a.PeriodID, From: at, Through: at, UploadBytes: 1}, {UserID: int(a.ProxyUserID), AccountingPeriod: a.PeriodID, From: at, Through: at.Add(-time.Second), UploadBytes: 1}}}
	if err := s.RecordXNodeTraffic(ctx, a.NodeID, "agent", 1, "bad", report); err == nil {
		t.Fatal("invalid batch accepted")
	}
	value, _ := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	sequence, _ := s.XNodeReportSequence(ctx, "agent")
	if sequence != 0 || value.PurchasedBytes != 0 || value.UsedBytes != 0 || value.BalanceMinor != 1000 {
		t.Fatal("partial financial commit", value, sequence)
	}
}
