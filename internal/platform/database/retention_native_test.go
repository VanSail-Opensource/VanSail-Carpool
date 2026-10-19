package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPurchaseCompactionRetainsNativeRefundAndUnsettledEvidence(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.Add(-60 * 24 * time.Hour)
	if _, err := s.AdjustBalance(ctx, identity.UserID, 1000, "seed", "retention-fixture", old); err != nil {
		t.Fatal(err)
	}
	combo, err := s.SaveCombo(ctx, ComboInput{Name: "Retention", ValidityDays: 30, PriceTXBMinor: 100, TrafficLimitBytes: 1000, ResetStrategy: "DAY", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	purchase, err := s.CreatePurchase(ctx, PurchaseInput{UserID: identity.UserID, ComboID: combo.ID, IdempotencyKey: "retention"}, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=? WHERE id=?`, stamp(old), purchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_periods SET purchase_id=?,status='closing',used_bytes=77 WHERE id=?`, purchase.ID, period.ID); err != nil {
		t.Fatal(err)
	}
	// Isolate the native accounting fence from the separate pending-job fence.
	if _, err = s.db.ExecContext(ctx, `DELETE FROM outbox_jobs`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	counts := map[string]int64{}
	if err = compactPurchasesTx(ctx, tx, now.Add(-7*24*time.Hour), now, counts); err != nil {
		t.Fatal("native evidence broke maintenance", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PurchaseByID(ctx, purchase.ID); err != nil || counts["purchases"] != 0 {
		t.Fatal("purchase-owned refund evidence was pruned", err)
	}
	retained, err := s.XNodePeriod(ctx, period.ID)
	if err != nil || retained.Status != "closing" || retained.UsedBytes != 77 {
		t.Fatal("unfinished accounting changed", retained, err)
	}
}

func TestNativeRetentionKeepsUnacknowledgedRateBoundaries(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES('agent','node',10,'fixture',?)`, stamp(now)); err != nil {
		t.Fatal(err)
	}
	for _, sequence := range []int{5, 11, 12} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_node_rates(agent_id,from_sequence,multiplier_fp) VALUES('agent',?,1000000)`, sequence); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	counts := map[string]int64{}
	if err = pruneNativeMaintenanceTx(ctx, tx, now.Add(-7*24*time.Hour), counts); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_node_rates WHERE agent_id='agent' AND from_sequence IN (5,11,12)`).Scan(&count); err != nil || count != 3 || counts["xnode_node_rates"] != 1 {
		t.Fatal("retention erased an accounting boundary", count, counts, err)
	}
}

func TestFleetUsageAggregatesPeriodsAndLegacyWithExactIntegers(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	second := xnode.Period{ID: "extra", ProxyUserID: identity.ProxyUserID, StartedAt: now, LimitBytes: 1000, Strategy: "DAY", Status: "open"}
	if err := s.SaveXNodePeriod(ctx, second); err != nil {
		t.Fatal(err)
	}
	first := int64(1<<53) + 1
	for index, id := range []string{period.ID, second.ID} {
		upload := first
		if index > 0 {
			upload = 7
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes,weighted_bytes) VALUES(?,'node',?,?,?,2,3)`, identity.ProxyUserID, id, now.Format(time.DateOnly), upload); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_legacy_history(proxy_user_id,node_id,date,raw_bytes,weighted_bytes) VALUES(?,'node',?,11,13)`, identity.ProxyUserID, now.Format(time.DateOnly)); err != nil {
		t.Fatal(err)
	}
	usage, err := s.XNodeFleetUsage(ctx, now, now)
	if err != nil || len(usage) != 1 || usage[0].UploadBytes != first+18 || usage[0].DownloadBytes != 4 || usage[0].WeightedBytes != 19 {
		t.Fatal("fleet aggregation changed exact accounting", usage, err)
	}
}
