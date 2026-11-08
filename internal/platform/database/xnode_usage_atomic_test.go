package database

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestUsageHistorySharesSingleAndBatchJournalDeduplication(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint("batch=", batch), func(t *testing.T) {
			s, identity, period, p, at := usageHistoryFixture(t)
			report := usageHistoryReport(identity, period, p, at, 1)
			for range 2 {
				if err := commitUsageHistoryFixture(t, s, report, batch); err != nil {
					t.Fatal(err)
				}
			}
			rows, coverage, err := s.XNodeUsageHistory(context.Background(), identity.UserID, at.Add(-time.Minute), at.Add(time.Second))
			if err != nil || len(rows) != 2 || coverage.Complete || coverage.Since == nil {
				t.Fatal("forward history/coverage missing or fabricated", rows, coverage, err)
			}
			for _, row := range rows {
				if row.BillingMode == "combo" && (row.ReferenceID != period.PurchaseID || row.UploadBytes != "3" || row.DownloadBytes != "4") {
					t.Fatal("combo history lost original purchase/raw bytes", row)
				}
				if row.BillingMode == "hourly" && (row.ReferenceID != p.EnrollmentID || row.UploadBytes != "7" || row.DownloadBytes != "9") {
					t.Fatal("hourly history duplicated native bytes or compared writer success", row)
				}
			}
			var generation interface{}
			if err = s.db.QueryRow(`SELECT account_generation FROM xnode_usage_minutes WHERE billing_mode='combo'`).Scan(&generation); err != nil || generation != nil {
				t.Fatal("unknown legacy generation was inferred", generation, err)
			}
			var credits int
			if err = s.db.QueryRow(`SELECT COUNT(*) FROM hourly_credit_reservations`).Scan(&credits); err != nil || credits != 0 {
				t.Fatal("raw presentation history mutated quota obligations", credits, err)
			}
		})
	}
}

func TestUsageHistoryWriteFailureRollsBackNativeCountersAndCursor(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint("batch=", batch), func(t *testing.T) {
			s, identity, period, p, at := usageHistoryFixture(t)
			ctx := context.Background()
			if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_usage_minutes ADD CONSTRAINT test_history_failure CHECK(billing_mode<>'hourly')`); err != nil {
				t.Fatal(err)
			}
			report := usageHistoryReport(identity, period, p, at, 1)
			if err := commitUsageHistoryFixture(t, s, report, batch); err == nil {
				t.Fatal("failed history storage received an accounting success")
			}
			for _, id := range []string{period.ID, p.PeriodID} {
				value, err := s.XNodePeriod(ctx, id)
				if err != nil || value.UsedBytes != 0 {
					t.Fatal("history failure left native accounting", value, err)
				}
			}
			sequence, _, err := s.XNodeStreamSequence(ctx, "agent", usageHistoryStream)
			if err != nil || sequence != 0 {
				t.Fatal("history failure advanced cursor", sequence, err)
			}
			var count int
			if err = s.db.QueryRow(`SELECT COUNT(*) FROM xnode_usage_minutes`).Scan(&count); err != nil || count != 0 {
				t.Fatal("history failure retained partial rows", count, err)
			}
			if _, err = s.db.ExecContext(ctx, `ALTER TABLE xnode_usage_minutes DROP CONSTRAINT test_history_failure`); err != nil {
				t.Fatal(err)
			}
			if err = commitUsageHistoryFixture(t, s, report, batch); err != nil {
				t.Fatal("original evidence did not replay after repair", err)
			}
		})
	}
}
