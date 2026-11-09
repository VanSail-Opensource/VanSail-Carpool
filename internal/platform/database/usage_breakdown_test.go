package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestUsageBreakdownUsesAcceptedRawJournalOnceAcrossBothModes(t *testing.T) {
	s, identity, period, p, at := usageHistoryFixture(t)
	report := usageHistoryReport(identity, period, p, at, 1)
	for range 2 {
		if err := commitUsageHistoryFixture(t, s, report, true); err != nil {
			t.Fatal(err)
		}
	}
	through := at.Add(time.Second)
	value, err := s.XNodeUsageBreakdown(context.Background(), identity.UserID, through.Add(-72*time.Hour), through)
	if err != nil || len(value.Nodes) != 1 || value.Nodes[0].UploadBytes != "3" || value.Nodes[0].DownloadBytes != "4" || len(value.HourlySquads) != 1 || value.HourlySquads[0].GroupID != p.GroupID || value.HourlySquads[0].UploadBytes != "7" || value.HourlySquads[0].DownloadBytes != "9" {
		t.Fatal("journal replay, multiplier or native hourly sample distorted client bytes", value, err)
	}
	if value.Coverage.Since == nil || value.Coverage.Complete {
		t.Fatal("new recording fabricated 72 hours", value.Coverage)
	}
	assertUsageBreakdownConservation(t, value)
}

func TestUsageBreakdownCombinesRawModesAndExactPurchaseAllocation(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	seconds := int64(7 * 24 * 60 * 60)
	usageBreakdownPurchase(t, s, u, "current", 100, &seconds, at)
	usageBreakdownExec(t, s, `INSERT INTO purchase_addons(purchase_id,group_id,charged_txb_minor) VALUES('current','paid-addon',40)`)
	usageBreakdownRaw(t, s, u, "combo", "current", "node-a", 1, 0, at)
	usageBreakdownRaw(t, s, u, "combo", "current", "node-b", 2, 0, at)
	usageBreakdownRaw(t, s, u, "combo", "previous", "node-b", 100, 0, at)
	usageBreakdownRaw(t, s, u, "hourly", "enrollment", "node-a", 3, 4, at)
	usageBreakdownExec(t, s, `INSERT INTO xnode_payg_node_minutes(user_id,node_id,minute,upload_bytes,download_bytes) VALUES($1,'node-a',$2,5,6)`, u.ID, at)
	// Catalog validity and paid addon rows must not reprice or duplicate the
	// already cumulative purchase charge retained in purchases.
	usageBreakdownExec(t, s, `UPDATE combos SET price_txb_minor=999999,validity_days=100 WHERE id=(SELECT combo_id FROM purchases WHERE id='current')`)
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at.Add(time.Minute))
	if err != nil || len(value.Nodes) != 2 {
		t.Fatal(value, err)
	}
	a, b := usageBreakdownNode(t, value, "node-a"), usageBreakdownNode(t, value, "node-b")
	if a.UploadBytes != "6" || a.DownloadBytes != "6" || b.UploadBytes != "102" || len(value.HourlySquads) != 1 || value.HourlySquads[0].GroupID != "enrollment" || value.HourlySquads[0].UploadBytes != "3" || value.HourlySquads[0].DownloadBytes != "4" {
		t.Fatal("raw modes lost or multiplied", value.Nodes)
	}
	if a.CostMinor != (model.RationalMinor{Numerator: "100", Denominator: "7"}) || b.CostMinor != (model.RationalMinor{Numerator: "200", Denominator: "7"}) {
		t.Fatal("estimate used other purchase traffic", value.Nodes)
	}
	if value.Cost.Legacy.Status != model.UsageLegacyAvailable || value.Cost.Legacy.AmountMinor == nil || *value.Cost.Legacy.AmountMinor != (model.RationalMinor{Numerator: "300", Denominator: "7"}) || value.Cost.ActualMinor != "0" {
		t.Fatal(value.Cost)
	}
	if value.PositiveNodes.Traffic != "2" || value.PositiveNodes.Cost != "2" || value.PositiveEntries.Traffic != "3" || value.PositiveEntries.Cost != "2" {
		t.Fatal(value.PositiveNodes, value.PositiveEntries)
	}
	assertUsageBreakdownConservation(t, value)
}

func TestUsageBreakdownRetainsExactCostOnlyRetiredNodesAndOwnerIsolation(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	a := usageBreakdownPAYGAccount(t, s, u, "retired", at)
	const price = int64(9007199254740993)
	usageBreakdownPAYGCharge(t, s, a, "cost-only", 0, 1, price, at)
	usageBreakdownPAYGCharge(t, s, a, "outside", 1, 1, 99, at.Add(-73*time.Hour))
	usageBreakdownExec(t, s, `UPDATE xnode_nodes SET enabled=0,retired_at=$2 WHERE id=$1`, a.NodeID, stamp(at))
	other := createTestUser(t, s, 89882)
	usageBreakdownRaw(t, s, other, "combo", "foreign", "foreign-node", 999, 999, at)
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at.Add(time.Minute))
	if err != nil || len(value.Nodes) != 1 || value.Nodes[0].UploadBytes != "0" || value.Nodes[0].NodeID != "retired" {
		t.Fatal(value, err)
	}
	if value.Cost.ActualMinor != "9007199254740993" || value.Nodes[0].CostMinor.Numerator != value.Cost.ActualMinor || value.PositiveNodes.Traffic != "0" || value.PositiveNodes.Cost != "1" {
		t.Fatal("cost became traffic/float/foreign owner's spend", value)
	}
	assertUsageBreakdownConservation(t, value)
}

func TestUsageBreakdownKnownZeroUnknownOriginalTermAndUnallocatedEstimate(t *testing.T) {
	for _, test := range []struct {
		name   string
		price  int64
		term   *int64
		status string
		amount string
	}{
		{"coupon-zero", 0, func() *int64 { v := int64(86400); return &v }(), model.UsageLegacyAvailable, "0"},
		{"known-no-traffic", 5, func() *int64 { v := int64(7 * 86400); return &v }(), model.UsageLegacyAvailable, "15"},
		{"old-unknown", 50, nil, model.UsageLegacyOriginalTermUnknown, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, u, at := usageBreakdownFixture(t)
			usageBreakdownPurchase(t, s, u, "current", test.price, test.term, at)
			value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at)
			if err != nil || len(value.Nodes) != 0 || value.Cost.Legacy.Status != test.status {
				t.Fatal(value, err)
			}
			if test.term == nil {
				if value.Cost.Legacy.AmountMinor != nil || value.Cost.KnownTotalMinor.Numerator != "0" {
					t.Fatal("unknown term invented an estimate", value.Cost)
				}
			} else if value.Cost.Legacy.AmountMinor == nil || value.Cost.Legacy.AmountMinor.Numerator != test.amount || value.Cost.UnallocatedMinor != *value.Cost.Legacy.AmountMinor {
				t.Fatal("unused estimate disappeared", value.Cost)
			}
			var stored interface{}
			if err = s.DB().QueryRow(`SELECT purchased_term_seconds FROM purchases WHERE id='current'`).Scan(&stored); err != nil || (test.term == nil && stored != nil) {
				t.Fatal("GET rewrote historical duration", stored, err)
			}
			assertUsageBreakdownConservation(t, value)
		})
	}
}
