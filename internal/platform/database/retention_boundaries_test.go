package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestCompactionKeepsFractionalGraceAndLiveScan(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 497725)
	now := time.Now().UTC().Truncate(time.Second)
	cutoff7, cutoff24 := now.Add(-7*24*time.Hour), now.Add(-24*time.Hour)
	order := createTestPaymentOrder(t, store, owner.ID, "stars", 100, now)
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET status='failed',updated_at=$1 WHERE id=$2`, stamp(cutoff7.Add(time.Nanosecond)), order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO activity_group_message_raw_events(chat_id,message_id,local_date,created_at) VALUES(-123,1,$1,$2)`, cutoff24.Format(time.DateOnly), stamp(cutoff24.Add(time.Nanosecond))); err != nil {
		t.Fatal(err)
	}
	scan, _, err := store.CreateConnectionScan(ctx, providerops.ConnectionScanInput{UserID: owner.ID, IdempotencyKey: "retention-live-scan", RequestFingerprint: "retention-live-scan-proof", ExpiresAt: now.Add(time.Nanosecond)}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateConnectionScan(ctx, scan.ID, providerops.ConnectionScanUpdate{Status: providerops.StatusFailed}, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	for iteration, expected := range []int64{0, 1, 0} {
		at := now
		if iteration > 0 {
			at = now.Add(2 * time.Nanosecond)
		}
		counts, err := store.CompactAndPrune(ctx, at.Add(-7*24*time.Hour), at.Add(-24*time.Hour), at)
		if err != nil || counts["payment_orders"] != expected || counts["group_message_facts"] != expected || counts["connection_scans"] != expected {
			t.Fatal("grace/TTL boundary compacted evidence early or twice", iteration, counts, err)
		}
	}
	var orders, facts int
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COALESCE(SUM(order_count),0) FROM payment_status_rollups),(SELECT COALESCE(SUM(group_message_count),0) FROM activity_daily_rollups)`).Scan(&orders, &facts); err != nil || orders != 1 || facts != 1 {
		t.Fatal("exact cutoff lost or doubled aggregate evidence", orders, facts, err)
	}
}

func TestRetentionDoesNotRefundLiveFinalNanosecondIPLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, owner, _, at := ipLookupFixture(t, 0)
	op := submitIP(t, service, owner, "retention-lease-boundary", false)
	now := at.Truncate(time.Second).Add(time.Hour)
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET created_at=$1 WHERE id=$2`, stamp(now.Add(-48*time.Hour)), op.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET lease_owner='retained-worker',heartbeat_at=$1,lease_expires_at=$2 WHERE id=(SELECT report_id FROM ip_lookup_checks WHERE operation_id=$3)`, stamp(now.Add(-5*time.Minute)), stamp(now.Add(time.Nanosecond)), op.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.Balance(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	check, err := store.IPLookupCheck(ctx, owner, op.ID)
	if err != nil || check.Refunded || check.Operation.Status != "queued" {
		t.Fatal("live lease was inferred failed and refunded", check, err)
	}
	balance, err := store.Balance(ctx, owner)
	if err != nil || balance.Minor != before.Minor {
		t.Fatal("live execution lost its debit", balance, err)
	}
	expired := now.Add(time.Nanosecond)
	if _, err := store.CompactAndPrune(ctx, expired.Add(-7*24*time.Hour), expired.Add(-24*time.Hour), expired); err != nil {
		t.Fatal(err)
	}
	check, err = store.IPLookupCheck(ctx, owner, op.ID)
	if err != nil || !check.Refunded || check.Operation.Status != "failed" {
		t.Fatal("expired unattempted execution did not recover its debit", check, err)
	}
	balance, err = store.Balance(ctx, owner)
	if err != nil || balance.MinorInt64()-before.MinorInt64() != 250 {
		t.Fatal("recovered debit differed from original charge", balance, err)
	}
}
