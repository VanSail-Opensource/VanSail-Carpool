package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestIPLookupCacheIsFreeWithRemainingAllowance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, _ := ipLookupFixture(t, 2)
	first := submitIP(t, service, user, "warm", false)
	finishIP(t, store, first.ID, false)
	before, err := store.Balance(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.State(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	q, err := service.Quote(ctx, user, "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	if q.Charge.Minor != "0" || q.UseQuota || q.CacheReportID == "" {
		t.Fatalf("cached quote=%+v", q)
	}
	op := submitIP(t, service, user, "free-cache", false)
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || !check.Cached || check.UsedQuota || check.Charge.Minor != "0" {
		t.Fatalf("cached check=%+v %v", check, err)
	}
	afterState, err := service.State(ctx, user)
	if err != nil || afterState.Allowance.Remaining != state.Allowance.Remaining {
		t.Fatalf("cache consumed allowance: %+v %v", afterState, err)
	}
	after, err := store.Balance(ctx, user)
	if err != nil || after.Minor != before.Minor {
		t.Fatal("cache deducted TXB")
	}
}

func TestIPLookupAnyProviderOutageKeepsCostAndCachesReport(t *testing.T) {
	t.Parallel()
	for _, quota := range []int{0, 1} {
		for _, outcome := range []string{"partial", "rejected", "all_errors"} {
			t.Run(string(rune('a'+quota))+outcome, func(t *testing.T) {
				ctx := context.Background()
				store, service, user, _, now := ipLookupFixture(t, quota)
				before, err := store.Balance(ctx, user)
				if err != nil {
					t.Fatal(err)
				}
				op := submitIP(t, service, user, "outage", false)
				r, _, err := store.IPLookupRun(ctx, op.ID)
				if err != nil {
					t.Fatal(err)
				}
				for i := range r.Checkpoint.Stages {
					if r.Checkpoint.Stages[i].Status == "queued" {
						r.Checkpoint.Stages[i].Status = "error"
						r.Checkpoint.Stages[i].Attempted = true
					}
				}
				r.Databases = []iplookup.Database{{ID: "ipapi", Status: "error"}}
				r.Status = "partial"
				r.Verdict = "inconclusive"
				if outcome == "rejected" {
					r.Status = "succeeded"
					r.Verdict = "unsuitable"
				}
				if outcome == "all_errors" {
					r.Status = "failed"
				}
				if err := store.FinishIPLookupRun(ctx, r, now); err != nil {
					t.Fatal(err)
				}
				if err := store.FinishIPLookupRun(ctx, r, now); err != nil {
					t.Fatal(err)
				}
				check, err := service.Check(ctx, user, op.ID)
				expectedCharge := "0"
				if quota == 0 {
					expectedCharge = "250"
				}
				if err != nil || check.Refunded || check.Charge.Minor != expectedCharge || check.Report == nil || check.Operation.Status == "failed" {
					t.Fatalf("outage result=%+v %v", check, err)
				}
				var itemStatus string
				if err := store.DB().QueryRowContext(ctx, `SELECT status FROM provider_operation_items WHERE operation_id=?`, op.ID).Scan(&itemStatus); err != nil || itemStatus != "succeeded" {
					t.Fatalf("delivered partial/rejected report item=%s error=%v", itemStatus, err)
				}
				state, err := service.State(ctx, user)
				if err != nil || state.Allowance.Remaining != 0 {
					t.Fatal("outage restored consumed quota")
				}
				after, err := store.Balance(ctx, user)
				expectedDebit := int64(0)
				if quota == 0 {
					expectedDebit = 250
				}
				if err != nil || before.MinorInt64()-after.MinorInt64() != expectedDebit {
					t.Fatal("outage refunded TXB")
				}
				preview, err := service.PreviewQuote(ctx, user, "8.8.8.8", false)
				if err != nil || preview.CachedReport == nil || preview.CachedReport.ID != r.ID || preview.CacheMatch != "exact" {
					t.Fatalf("outage did not become cache: %+v %v", preview, err)
				}
			})
		}
	}
}

func TestIPLookupLatestCacheHandlesFractionalTimestampOrdering(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	first := submitIP(t, service, user, "whole-second", false)
	r, _, err := store.IPLookupRun(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	r.Status, r.Verdict = "succeeded", "suitable"
	wholeSecond := now.Truncate(time.Second)
	if err := store.FinishIPLookupRun(ctx, r, wholeSecond); err != nil {
		t.Fatal(err)
	}
	refresh := submitIP(t, service, user, "fractional-refresh", true)
	newReport, _, err := store.IPLookupRun(ctx, refresh.ID)
	if err != nil {
		t.Fatal(err)
	}
	newReport.Status, newReport.Verdict = "succeeded", "suitable"
	stage, claimed, err := store.ClaimIPLookupLiveStage(ctx, newReport.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		stage.Status = "succeeded"
		if err := store.FinishIPLookupLiveStage(ctx, newReport.ID, stage); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.FinishIPLookupRun(ctx, newReport, wholeSecond.Add(time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	q, err := service.Quote(ctx, user, "8.8.8.8", false)
	if err != nil || q.CacheReportID != newReport.ID {
		t.Fatalf("latest report selected incorrectly: %+v %v", q, err)
	}
}
