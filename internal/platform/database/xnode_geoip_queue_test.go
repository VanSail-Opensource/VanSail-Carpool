package database

import (
	"context"
	"testing"
)

func TestQueueXNodeGeoIPCoalescesAndReopensFailedDelivery(t *testing.T) {
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
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*),MIN(id) FROM outbox_jobs WHERE kind='xnode_firewall' AND json_extract(payload,'$.nodeId')='node' AND json_extract(payload,'$.bank')='geoip'`).Scan(&count, &id); err != nil || count != 1 {
		t.Fatalf("coalesced GeoIP jobs = %d, err=%v", count, err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueXNodeGeoIP(ctx, "node"); err != nil {
		t.Fatal(err)
	}
	var status string
	var attempts int
	if err := store.db.QueryRowContext(ctx, `SELECT status,attempts FROM outbox_jobs WHERE id=?`, id).Scan(&status, &attempts); err != nil || status != "pending" || attempts != 0 {
		t.Fatalf("retried GeoIP job = %q/%d, err=%v", status, attempts, err)
	}
}
