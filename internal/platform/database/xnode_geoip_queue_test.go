package database

import (
	"context"
	"testing"
)

func TestQueueXNodeGeoIPCoalescesAndPreservesFailedDelivery(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.QueueXNodeGeoIP(ctx, "node"); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueXNodeGeoIP(ctx, "node"); err != nil {
		t.Fatal(err)
	}
	var count int
	var id string
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*),MIN(id) FROM outbox_jobs WHERE kind='xnode_firewall' AND (CASE ((payload)::jsonb #> '{nodeId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{nodeId}' END)='node' AND (CASE ((payload)::jsonb #> '{bank}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{bank}' END)='geoip'`).Scan(&count, &id); err != nil || count != 1 {
		t.Fatalf("coalesced GeoIP jobs = %d, err=%v", count, err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.QueueXNodeGeoIP(ctx, "node"); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	var attempts int
	if err := store.db.QueryRowContext(ctx, `SELECT status,attempts FROM outbox_jobs WHERE id=$1`, id).Scan(&status, &attempts); err != nil || status != "failed" || attempts != 10 {
		t.Fatalf("historical GeoIP job = %q/%d, err=%v", status, attempts, err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_firewall' AND status='pending' AND attempts=0 AND id<>$1`, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("fresh GeoIP delivery jobs = %d, err=%v", count, err)
	}
}
