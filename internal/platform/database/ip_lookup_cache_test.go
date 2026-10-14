package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func insertIPCache(t *testing.T, store *Store, id, ip, status string, completed time.Time) iplookup.Report {
	t.Helper()
	c := iplookup.DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "ipapi"
	}
	r := iplookup.NewReport(id, ip, c)
	// This fixture completes quality stages; RPKI policy has separate hosted cases.
	r.Checkpoint.RPKI.Status = "success"
	negative := false
	iplookup.RecordProvider(&r, iplookup.ProviderResult{ID: "ipapi", Status: "success", Complete: true, Facts: iplookup.Facts{Country: "JP", City: "Tokyo", NetworkType: "residential"}, Signals: iplookup.Signals{Abuse: &negative, Datacenter: &negative, VPN: &negative, Proxy: &negative, Tor: &negative}}, c)
	r = iplookup.Aggregate(r, completed)
	r.Status = status
	raw, err := iplookup.EncodeReport(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(context.Background(), `INSERT INTO ip_lookup_reports(id,ip,status,report_json,created_at,completed_at) VALUES(?,?,?,?,?,?)`, id, ip, status, string(raw), stamp(completed.Add(-time.Second)), stamp(completed)); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestIPLookupPreviewPrefersExactThenNewestCompletedSubnet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 2)
	insertIPCache(t, store, "newest-completion", "8.8.8.11", "partial", now.Add(2*time.Second))
	insertIPCache(t, store, "newest-insertion", "8.8.8.12", "succeeded", now.Add(time.Second))
	insertIPCache(t, store, "older-exact", "8.8.8.13", "succeeded", now)
	insertIPCache(t, store, "ipv6-exact", "2606:4700:4700::1111", "succeeded", now)
	insertIPCache(t, store, "legacy-failed", "8.8.9.14", "failed", now)
	wholeSecond := now.Truncate(time.Second)
	insertIPCache(t, store, "later-nanoseconds", "8.8.10.11", "succeeded", wholeSecond.Add(200*time.Nanosecond))
	insertIPCache(t, store, "earlier-nanoseconds", "8.8.10.12", "succeeded", wholeSecond.Add(100*time.Nanosecond))
	insertIPCache(t, store, "later-exact-nanoseconds", "8.8.11.11", "succeeded", wholeSecond.Add(200*time.Nanosecond))
	insertIPCache(t, store, "earlier-exact-nanoseconds", "8.8.11.11", "succeeded", wholeSecond.Add(100*time.Nanosecond))
	insertIPCache(t, store, "equal-first", "8.8.12.11", "succeeded", wholeSecond)
	insertIPCache(t, store, "equal-last", "8.8.12.12", "succeeded", wholeSecond)
	for _, tc := range []struct{ ip, reportID, match string }{
		{"8.8.8.13", "older-exact", "exact"},
		{"8.8.8.14", "newest-completion", "subnet"},
		{"8.8.9.13", "", "none"},
		{"8.8.9.14", "", "none"},
		{"8.8.10.13", "later-nanoseconds", "subnet"},
		{"8.8.11.11", "later-exact-nanoseconds", "exact"},
		{"8.8.12.13", "equal-last", "subnet"},
		{"2606:4700:4700::1111", "ipv6-exact", "exact"},
		{"2606:4700:4700::1112", "", "none"},
	} {
		t.Run(tc.ip, func(t *testing.T) {
			preview, err := service.PreviewQuote(ctx, user, tc.ip, false)
			if err != nil || preview.CacheReportID != tc.reportID || preview.CacheMatch != tc.match {
				t.Fatalf("preview=%+v %v", preview, err)
			}
			if tc.reportID != "" && (preview.CachedReport == nil || preview.CachedReport.ID != tc.reportID || preview.UseQuota || preview.Charge.Minor != "0") {
				t.Fatalf("preview lost compact free result: %+v", preview)
			}
		})
	}
	var operations int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations WHERE kind=?`, iplookup.OperationKind).Scan(&operations); err != nil || operations != 0 {
		t.Fatal("cache preview created an operation")
	}
	state, err := service.State(ctx, user)
	if err != nil || state.Allowance.Remaining != 2 {
		t.Fatal("cache preview consumed quota")
	}
}

func TestIPLookupQuoteBecomingSubnetCacheIsFreeAndRestoresRequestedIP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 2)
	quote, err := service.PreviewQuote(ctx, user, "8.8.8.8", false)
	if err != nil || !quote.UseQuota || quote.CacheMatch != "none" {
		t.Fatalf("initial quote=%+v %v", quote, err)
	}
	insertIPCache(t, store, "became-cache", "8.8.8.9", "succeeded", now)
	op, err := service.Submit(ctx, user, "became-subnet-cache", quote.Quote)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Submit(ctx, user, "became-subnet-cache", quote.Quote)
	if err != nil || replay.ID != op.ID {
		t.Fatal("cache command replay was not idempotent")
	}
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || !check.Cached || check.UsedQuota || check.Charge.Minor != "0" || check.RequestedIP != "8.8.8.8" || check.CacheMatch != "subnet" || check.Report == nil || check.Report.IP != "8.8.8.9" {
		t.Fatalf("cache receipt=%+v %v", check, err)
	}
	state, err := service.State(ctx, user)
	if err != nil || state.Allowance.Remaining != 2 {
		t.Fatal("new cache consumed allowance")
	}
}

func TestIPLookupPreviewDoesNotSignDisplayFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	insertIPCache(t, store, "signed-preview", "8.8.8.8", "succeeded", now)
	preview, err := service.PreviewQuote(ctx, user, "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	preview.CacheMatch = "none"
	preview.CachedReport = nil
	if _, err := service.Submit(ctx, user, "signed-base-only", preview.Quote); err != nil {
		t.Fatalf("display fields affected quote signature: %v", err)
	}
	preview.IP = "8.8.8.9"
	if _, err := service.Submit(ctx, user, "tampered-base", preview.Quote); iplookup.ErrorCode(err) != "IP_LOOKUP_QUOTE_CHANGED" {
		t.Fatal("signed submission fields were mutable")
	}
}
