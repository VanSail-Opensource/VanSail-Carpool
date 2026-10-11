package database

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"strings"
	"testing"
	"time"
)

func TestIPLookupLeasesAndSharedProgressKeepReceiptPrivate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 2)
	op := submitIP(t, service, user, "background", false)
	claimed, err := store.ClaimIPLookupLease(ctx, op.ID, "first-worker", now)
	if err != nil || !claimed {
		t.Fatalf("claim %v %v", claimed, err)
	}
	claimed, err = store.ClaimIPLookupLease(ctx, op.ID, "second-worker", now.Add(30*time.Second))
	if err != nil || claimed {
		t.Fatalf("live lease stolen: %v %v", claimed, err)
	}
	claimed, err = store.ClaimIPLookupLease(ctx, op.ID, "second-worker", now.Add(61*time.Second))
	if err != nil || !claimed {
		t.Fatalf("expired lease not recovered: %v %v", claimed, err)
	}
	if err := store.HeartbeatIPLookup(ctx, op.ID, "first-worker", now.Add(62*time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale worker renewed: %v", err)
	}
	shared, err := service.Progress(ctx, "8.8.8.8")
	if err != nil || shared == nil || shared.IP != "8.8.8.8" {
		t.Fatalf("progress %+v %v", shared, err)
	}
	raw, err := json.Marshal(shared)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{user, op.ID, "charge", "quota", "allowance", "credential"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("private progress field %q", private)
		}
	}
	q, err := service.Quote(ctx, user, "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Submit(ctx, user, "duplicate-background", q); iplookup.ErrorCode(err) != "IP_LOOKUP_BUSY" {
		t.Fatalf("duplicate accepted: %v", err)
	}
	state, err := service.State(ctx, user)
	if err != nil || state.Allowance.Remaining != 1 {
		t.Fatalf("duplicate consumed quota: %+v %v", state, err)
	}
}

func TestIPLookupRefreshConsumesConfiguredUnitsAndRefundsOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, _ := ipLookupFixture(t, 8)
	op := submitIP(t, service, user, "initial", false)
	_, c, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	finishIP(t, store, op.ID, false)
	c.RefreshQuotaCost = 3
	if err := store.SaveIPLookupSettings(ctx, user, c, nil, nil); err != nil {
		t.Fatal(err)
	}
	q, err := service.Quote(ctx, user, "8.8.8.8", true)
	if err != nil || !q.UseQuota || q.QuotaCost != 3 || q.Charge.Minor != "0" {
		t.Fatalf("quote %+v %v", q, err)
	}
	refresh, err := service.Submit(ctx, user, "three-units", q)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.State(ctx, user)
	if err != nil || state.Allowance.Remaining != 4 {
		t.Fatalf("debit %+v %v", state, err)
	}
	report := finishIP(t, store, refresh.ID, true)
	if err := store.FinishIPLookupRun(ctx, report, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	state, err = service.State(ctx, user)
	if err != nil || state.Allowance.Remaining != 7 {
		t.Fatalf("refund %+v %v", state, err)
	}
	check, err := service.Check(ctx, user, refresh.ID)
	if err != nil || check.QuotaConsumed != 3 || !check.Refunded {
		t.Fatalf("receipt %+v %v", check, err)
	}
	c.RefreshQuotaCost = 8
	if err := store.SaveIPLookupSettings(ctx, user, c, nil, nil); err != nil {
		t.Fatal(err)
	}
	q, err = service.Quote(ctx, user, "8.8.8.8", true)
	if err != nil || q.UseQuota || q.QuotaCost != 0 || q.Charge.Minor != "350" {
		t.Fatalf("fallback %+v %v", q, err)
	}
}

func TestCombinedIPCacheReplacementPreservesReceipt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	first := submitIP(t, service, user, "first-combined", false)
	r, _, err := store.IPLookupRun(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	r.Status, r.Verdict = "succeeded", "suitable"
	r.LiveDetails = &iplookup.LiveDetails{IP: r.IP, SourceIP: r.IP}
	if err := store.FinishIPLookupRun(ctx, r, now); err != nil {
		t.Fatal(err)
	}
	q, err := service.Quote(ctx, user, "8.8.8.9", true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Submit(ctx, user, "neighbor-combined", q)
	if err != nil {
		t.Fatal(err)
	}
	newer, _, err := store.IPLookupRun(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	newer.Status, newer.Verdict = "succeeded", "suitable"
	newer.LiveDetails = &iplookup.LiveDetails{IP: newer.IP, SourceIP: newer.IP}
	if err := store.FinishIPLookupRun(ctx, newer, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_lookup_reports WHERE status IN ('succeeded','partial')`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cache duplicates: %d %v", count, err)
	}
	receipt, err := service.Check(ctx, user, first.ID)
	if err != nil || receipt.Report != nil || receipt.Charge.Minor != "250" || receipt.RequestedIP != "8.8.8.8" {
		t.Fatalf("receipt lost %+v %v", receipt, err)
	}
	preview, err := service.PreviewQuote(ctx, user, "8.8.8.8", false)
	if err != nil || preview.CachedReport == nil || preview.CachedReport.ID != newer.ID || preview.CacheMatch != "subnet" || preview.Charge.Minor != "0" {
		t.Fatalf("replacement %+v %v", preview, err)
	}
}
