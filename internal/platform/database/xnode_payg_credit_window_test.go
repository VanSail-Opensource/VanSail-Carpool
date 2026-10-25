package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func paygReportAt(t *testing.T, s *Store, a xnode.PAYGAccount, sequence uint64, at time.Time, bytes int64) {
	t.Helper()
	report := wire.TrafficReport{ID: fmt.Sprintf("agent:%d", sequence), Samples: []wire.TrafficSample{{UserID: int(a.ProxyUserID), AccountingPeriod: a.PeriodID, From: at, Through: at, DownloadBytes: uint64(bytes)}}}
	for range 2 {
		if err := s.RecordXNodeTraffic(context.Background(), a.NodeID, "agent", sequence, report.ID, report); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPAYGRollingWindowExcludesExactlyExpiredUnbilledTraffic(t *testing.T) {
	s, _, _, a := paygBillingFixture(t)
	ctx := context.Background()
	end := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	start := end.Add(-72 * time.Hour)
	const half = int64(50 << 20)
	paygReportAt(t, s, a, 1, start, half)
	paygReportAt(t, s, a, 2, end, half)
	value, err := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.PurchasedBytes != 0 || value.UsedBytes != 2*half || value.BalanceMinor != 1000 {
		t.Fatal("expired probes counted toward rolling threshold", value, err)
	}
	paygReportAt(t, s, a, 3, end.Add(time.Nanosecond), half)
	value, err = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.PurchasedBytes != xnode.PAYGGB || value.CreditBytes != xnode.PAYGGB-2*half || value.UsedBytes != 3*half || value.BalanceMinor != 900 {
		t.Fatal("window charge lost raw evidence or billed aged-out bytes", value, err)
	}
	var charges int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_payg_charges WHERE proxy_user_id=?`, a.ProxyUserID).Scan(&charges); err != nil || charges != 1 {
		t.Fatal("replayed report billed again", charges, err)
	}
}

func TestPAYGPrepaidRemainderExpiresWithoutIdleReplacementCharge(t *testing.T) {
	s, _, _, a := paygBillingFixture(t)
	ctx := context.Background()
	expires := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	started := expires.Add(-72 * time.Hour)
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_payg_accounts SET purchased_bytes=?,credit_bytes=?,credit_started_at=? WHERE proxy_user_id=?`, xnode.PAYGGB, xnode.PAYGGB, stamp(started), a.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	// Delayed metering before the exact expiry may consume already-paid bytes.
	paygReportAt(t, s, a, 1, expires.Add(-time.Nanosecond), 99<<20)
	value, err := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.CreditBytes != xnode.PAYGGB-(99<<20) || !value.HasCredit(expires.Add(-time.Nanosecond)) || value.HasCredit(expires) {
		t.Fatal("credit lifetime or delayed usage boundary", value, err)
	}
	if err := s.ObserveXNodePAYGConnections(ctx, a.NodeID, []int64{a.ProxyUserID}, expires); err != nil {
		t.Fatal(err)
	}
	value, _ = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if value.PurchasedBytes != xnode.PAYGGB || value.BalanceMinor != 1000 {
		t.Fatal("idle expired credit purchased a new GB", value)
	}
	paygReportAt(t, s, a, 2, expires, 1<<20)
	value, err = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.PurchasedBytes != 2*xnode.PAYGGB || value.CreditBytes != xnode.PAYGGB-(1<<20) || value.BalanceMinor != 900 {
		t.Fatal("expired unused bytes leaked into replacement GB", value, err)
	}
	if value.CreditStartedAt.Before(expires) {
		t.Fatal("replacement kept expired purchase timestamp", value)
	}
}

func TestPAYGTrafficIgnoresNodeMultiplierAndRateChanges(t *testing.T) {
	s, _, combo, a := paygBillingFixture(t)
	ctx := context.Background()
	for index, rate := range []int64{0, 500_000, 100_000_000} {
		if err := s.SetXNodeRate(ctx, a.NodeID, "agent", uint64(index), rate); err != nil {
			t.Fatal(err)
		}
		paygReport(t, s, a, uint64(index+1), 1<<20)
	}
	paygReport(t, s, a, 4, uint64(xnode.PAYGStartBytes-(3<<20)))
	value, err := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.UsedBytes != 100<<20 || value.PurchasedBytes != xnode.PAYGGB || value.CreditBytes != xnode.PAYGGB-(100<<20) || value.BalanceMinor != 900 {
		t.Fatal("PAYG applied a node multiplier", value, err)
	}
	period, err := s.XNodePeriod(ctx, combo.ID)
	if err != nil || period.UsedBytes != 0 {
		t.Fatal("PAYG affected weighted combo quota", period, err)
	}
	var rate int64
	if err := s.DB().QueryRowContext(ctx, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='agent' ORDER BY from_sequence DESC LIMIT 1`).Scan(&rate); err != nil || rate != 100_000_000 {
		t.Fatal("PAYG changed configured multiplier", rate, err)
	}
}
