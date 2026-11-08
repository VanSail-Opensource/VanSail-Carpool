package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCompletedPurgeSuppressesLateHourlyPresentationButAcceptsJournal(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint("batch=", batch), func(t *testing.T) {
			s, identity, period, p, at := usageHistoryFixture(t)
			ctx := context.Background()
			proof := settledUsageCredit(t, s, identity.UserID, at)
			prepareUsagePurge(t, s, identity.UserID, at)
			if err := s.CompleteAccountPurge(ctx, identity.UserID, 1, at); err != nil {
				t.Fatal(err)
			}
			var ledgerBefore, receiptsBefore int
			if err := s.DB().QueryRowContext(ctx, `SELECT (SELECT count(*) FROM ledger_entries),(SELECT count(*) FROM hourly_settlements)`).Scan(&ledgerBefore, &receiptsBefore); err != nil {
				t.Fatal(err)
			}
			for _, sequence := range []uint64{1, 2, 2} {
				report := usageHistoryReport(identity, period, p, at, sequence)
				report.Samples = nil
				if err := commitUsageHistoryFixture(t, s, report, batch); err != nil {
					t.Fatal("late receipt stopped journal acceptance", err)
				}
			}
			sequence, _, err := s.XNodeStreamSequence(ctx, "agent", usageHistoryStream)
			if err != nil || sequence != 2 {
				t.Fatal("cursor/replay changed", sequence, err)
			}
			var rows, ledgerAfter, receiptsAfter int
			if err = s.DB().QueryRowContext(ctx, `SELECT (SELECT count(*) FROM xnode_usage_minutes WHERE owner_id=$1),(SELECT count(*) FROM ledger_entries),(SELECT count(*) FROM hourly_settlements)`, identity.UserID).Scan(&rows, &ledgerAfter, &receiptsAfter); err != nil || rows != 0 || ledgerAfter != ledgerBefore || receiptsAfter != receiptsBefore {
				t.Fatal("presentation returned or financial evidence changed", rows, ledgerAfter, receiptsAfter, err)
			}
			settled, err := s.ApplyHourlyCreditProof(ctx, proof.NodeID, proof.AgentID, proof)
			if err != nil || settled.FinalProofDigest != proof.Digest || settled.AdmittedUploadBytes != 3 || settled.AdmittedDownloadBytes != 4 || settled.UnusedBytes != 3 {
				t.Fatal("presentation suppression changed retained credit proof replay", settled, err)
			}
		})
	}
}

func TestCompletedHigherGenerationPurgeSuppressesOldHistoryAfterRejoin(t *testing.T) {
	s, identity, period, old, at := usageHistoryFixture(t)
	ctx := context.Background()
	// Earlier admin recovery can skip lifecycle generations without completing
	// a purge. The actual completed purge is 5, not sample generation 0 plus1.
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET account_generation=4 WHERE id=$1`, identity.UserID); err != nil {
		t.Fatal(err)
	}
	prepareUsagePurge(t, s, identity.UserID, at)
	if err := s.CompleteAccountPurge(ctx, identity.UserID, 5, at); err != nil {
		t.Fatal(err)
	}
	owner, err := s.UserByID(ctx, identity.UserID)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err = s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: owner.TelegramID}, false)
	if err != nil || owner.AccountGeneration != 5 {
		t.Fatal(owner, err)
	}
	next := nextUsageEnrollment(t, s, owner, at)
	report := usageHistoryReport(identity, period, old, at, 1)
	report.Samples = nil
	newSample := report.HourlySamples[0]
	newSample.Owner.Generation = uint64(next.AccountGeneration)
	newSample.EnrollmentID = next.EnrollmentID
	newSample.UserID = int(next.ProxyUserID)
	newSample.AccountingPeriod = next.PeriodID
	report.HourlySamples = append(report.HourlySamples, newSample)
	for range 2 {
		if err = commitUsageHistoryFixture(t, s, report, true); err != nil {
			t.Fatal("mixed eligible journal failed", err)
		}
	}
	rows, _, err := s.XNodeUsageHistory(ctx, identity.UserID, at.Add(-time.Minute), at.Add(time.Second))
	if err != nil || len(rows) != 1 || rows[0].ReferenceID != next.EnrollmentID || rows[0].UploadBytes != "7" || rows[0].DownloadBytes != "9" {
		t.Fatal("old history returned or new generation was suppressed", rows, err)
	}
}

func TestVisibleUsageGenerationConflictStillRejectsWholeJournal(t *testing.T) {
	s, identity, period, p, at := usageHistoryFixture(t)
	ctx := context.Background()
	report := usageHistoryReport(identity, period, p, at, 1)
	report.Samples = nil
	if err := commitUsageHistoryFixture(t, s, report, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_usage_minutes SET account_generation=1 WHERE owner_id=$1`, identity.UserID); err != nil {
		t.Fatal(err)
	}
	report = usageHistoryReport(identity, period, p, at, 2)
	report.Samples = nil
	if err := commitUsageHistoryFixture(t, s, report, true); !errors.Is(err, ErrConflict) {
		t.Fatal("eligible generation conflict was silently acknowledged", err)
	}
	if sequence, _, err := s.XNodeStreamSequence(ctx, "agent", usageHistoryStream); err != nil || sequence != 1 {
		t.Fatal("failed materialization advanced cursor", sequence, err)
	}
}
