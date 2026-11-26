package database

import (
	"context"
	"fmt"
	"slices"
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
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_payg_charges WHERE proxy_user_id=$1`, a.ProxyUserID).Scan(&charges); err != nil || charges != 1 {
		t.Fatal("replayed report billed again", charges, err)
	}
}

func TestPAYGPrepaidRemainderExpiresWithoutIdleReplacementCharge(t *testing.T) {
	s, _, _, a := paygBillingFixture(t)
	ctx := context.Background()
	expires := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	started := expires.Add(-72 * time.Hour)
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_payg_accounts SET purchased_bytes=$1,credit_bytes=$2,credit_started_at=$3 WHERE proxy_user_id=$4`, xnode.PAYGGB, xnode.PAYGGB, stamp(started), a.ProxyUserID); err != nil {
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
	if err := s.DB().QueryRowContext(ctx, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='agent' ORDER BY from_sequence DESC LIMIT 1`).Scan(&rate); err != nil || rate != xnode.MultiplierScale {
		t.Fatal("retired multiplier was not coerced to raw bytes", rate, err)
	}
}

func TestPAYGWindowKeepsNewestExactThresholdAndPendingEvidence(t *testing.T) {
	s, _, _, account := paygBillingFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	from := paygWindowStamp(now.Add(-xnode.PAYGWindow))
	const mib int64 = 1 << 20
	samples := []struct {
		at                time.Time
		traffic, unbilled int64
	}{
		{now.Add(-xnode.PAYGWindow), 5 * mib, 5 * mib}, // expired even if pending
		{now.Add(-5 * time.Minute), 2 * mib, 2 * mib},  // pending, still in-window
		{now.Add(-4 * time.Minute), 8 * mib, 0},        // unnecessary
		{now.Add(-3 * time.Minute), 0, 0},              // zero after the exact excess
		{now.Add(-2 * time.Minute), 40 * mib, 0},       // oldest needed sample
		{now, 60 * mib, 0},                             // inclusive upper boundary
		{now.Add(time.Nanosecond), 90 * mib, 0},        // outside the current window
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, sample := range samples {
		if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through,traffic_bytes,unbilled_bytes) VALUES($1,$2,$3,$4)`, account.ProxyUserID, paygWindowStamp(sample.at), sample.traffic, sample.unbilled); err != nil {
			t.Fatal(err)
		}
	}
	traffic, pending, cutoff, err := loadPAYGWindow(ctx, tx, account.ProxyUserID, from, paygWindowStamp(now.Add(-time.Nanosecond)))
	if err != nil || traffic != 50*mib || pending != 2*mib || cutoff.Valid {
		t.Fatalf("sub-threshold window: traffic=%d pending=%d cutoff=%v err=%v", traffic, pending, cutoff, err)
	}
	traffic, pending, cutoff, err = loadPAYGWindow(ctx, tx, account.ProxyUserID, from, paygWindowStamp(now))
	if err != nil || traffic != 110*mib || pending != 2*mib || !cutoff.Valid || cutoff.String != paygWindowStamp(samples[4].at) {
		t.Fatalf("funded window: traffic=%d pending=%d cutoff=%v err=%v", traffic, pending, cutoff, err)
	}
	if err := prunePAYGWindow(ctx, tx, account.ProxyUserID, from, cutoff); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT through FROM xnode_payg_windows WHERE proxy_user_id=$1 ORDER BY through`, account.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var kept []string
	for rows.Next() {
		var through string
		if err := rows.Scan(&through); err != nil {
			t.Fatal(err)
		}
		kept = append(kept, through)
	}
	want := []string{paygWindowStamp(samples[1].at), paygWindowStamp(samples[4].at), paygWindowStamp(samples[5].at), paygWindowStamp(samples[6].at)}
	if err := rows.Err(); err != nil || !slices.Equal(kept, want) {
		t.Fatalf("kept %v, want %v: %v", kept, want, err)
	}
}

func TestPAYGWindowPruneFallsBackBeyondBoundedPrefix(t *testing.T) {
	s, _, _, account := paygBillingFixture(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	const mib int64 = 1 << 20
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through,traffic_bytes)
 SELECT $1,to_char($2::timestamp + pos*interval '1 second','YYYY-MM-DD"T"HH24:MI:SS.US"000Z"'),$3
 FROM generate_series(0,229) AS pos`, account.ProxyUserID, start, mib)
	if err != nil {
		t.Fatal(err)
	}
	traffic, pending, cutoff, err := loadPAYGWindow(ctx, tx, account.ProxyUserID, paygWindowStamp(start.Add(-time.Nanosecond)), paygWindowStamp(start.Add(229*time.Second)))
	want := paygWindowStamp(start.Add(130 * time.Second))
	if err != nil || traffic != 230*mib || pending != 0 || !cutoff.Valid || cutoff.String != want {
		t.Fatalf("fallback: traffic=%d pending=%d cutoff=%v (want %s) err=%v", traffic, pending, cutoff, want, err)
	}
}
