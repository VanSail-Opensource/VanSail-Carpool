package database

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestIPLookupCompactMigrationPreservesPendingStagesAndHistoricalAccounting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, _ := ipLookupFixture(t, 0)
	op := submitIP(t, service, user, "legacy-pending", false)
	pending, _, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	legacyPending := fmt.Sprintf(`{"id":%q,"ip":"8.8.8.8","status":"processing","verdict":"inconclusive","reasons":[],"facts":{},"sources":{},"policyVersion":"residential-v2","parserVersion":"residential-v2","providers":[{"id":"abuseipdb","status":"success","complete":true,"facts":{"country":"JP","city":"Tokyo"},"scores":{},"signals":{"abuse":false}},{"id":"ipapi","status":"processing","facts":{},"scores":{},"signals":{}},{"id":"maxmind","status":"queued","facts":{},"scores":{},"signals":{}}]}`, pending.ID)
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET report_json=? WHERE id=?`, legacyPending, pending.ID); err != nil {
		t.Fatal(err)
	}
	failedQuote, err := service.Quote(ctx, user, "8.8.9.8", false)
	if err != nil {
		t.Fatal(err)
	}
	failedOp, err := service.Submit(ctx, user, "legacy-refunded", failedQuote)
	if err != nil {
		t.Fatal(err)
	}
	failed := finishIP(t, store, failedOp.ID, true)
	legacyFailed := fmt.Sprintf(`{"id":%q,"ip":"8.8.9.8","status":"failed","verdict":"inconclusive","refundRequired":true,"reasons":["all_providers_failed"],"facts":{},"sources":{},"policyVersion":"residential-v2","parserVersion":"residential-v2","providers":[{"id":"ipapi","status":"error","errorCode":"IP_LOOKUP_PROVIDER_UNAVAILABLE","facts":{},"scores":{},"signals":{}}]}`, failed.ID)
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET report_json=? WHERE id=?`, legacyFailed, failed.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.Balance(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	var ledgerBefore int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=?`, user).Scan(&ledgerBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `DELETE FROM schema_migrations WHERE version='067_ip_lookup_compact.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	resumed, _, err := store.IPLookupRun(ctx, op.ID)
	if err != nil || resumed.Checkpoint == nil || resumed.Facts.Country != "JP" || !iplookup.ReportAttempted(resumed) {
		t.Fatalf("pending migration=%+v %v", resumed, err)
	}
	stages := map[string]string{}
	for _, stage := range resumed.Checkpoint.Stages {
		stages[stage.ID] = stage.Status
	}
	if stages["abuseipdb"] != "success" || stages["ipapi"] != "processing" || stages["maxmind"] != "queued" {
		t.Fatalf("completed/ambiguous/unattempted stages changed: %v", stages)
	}
	for _, id := range []string{pending.ID, failed.ID} {
		var raw string
		if err := store.DB().QueryRowContext(ctx, `SELECT report_json FROM ip_lookup_reports WHERE id=?`, id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(raw, `"providers"`) || strings.Contains(raw, `"scores"`) || strings.Contains(raw, `"signals"`) {
			t.Fatalf("per-database payload survived: %s", raw)
		}
		decoded, err := iplookup.DecodeReport([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if id == failed.ID && (decoded.Status != "failed" || decoded.PolicyVersion != "residential-v2" || !decoded.RefundRequired) {
			t.Fatalf("historical failure rewritten: %+v", decoded)
		}
	}
	after, err := store.Balance(ctx, user)
	if err != nil || before.Minor != after.Minor {
		t.Fatal("compaction changed balance")
	}
	var ledgerAfter int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=?`, user).Scan(&ledgerAfter); err != nil || ledgerAfter != ledgerBefore {
		t.Fatal("compaction changed ledger")
	}
	preview, err := service.PreviewQuote(ctx, user, "8.8.9.8", false)
	if err != nil || preview.CachedReport != nil {
		t.Fatal("legacy failed report became a cache hit")
	}
	check, err := service.Check(ctx, user, failedOp.ID)
	if err != nil || !check.Refunded || check.Operation.Status != "failed" || check.Charge.Minor != "0" {
		t.Fatalf("historical refunded receipt changed: %+v %v", check, err)
	}
}
