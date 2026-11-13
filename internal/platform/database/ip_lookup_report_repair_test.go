package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func loadRepairReport(t *testing.T, store *Store, id string) iplookup.Report {
	t.Helper()
	report, _, err := store.IPLookupReportByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestConcurrentPartialReportRepairsPreserveWinningEvidence(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	completed := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	insertIPCache(t, store, "report-repair-cas", "8.8.8.8", "partial", completed)
	// The token must cover stored bytes, including legacy whitespace, rather
	// than a re-encoded normalized projection.
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET report_json=' '||report_json||' ' WHERE id=$1`, "report-repair-cas"); err != nil {
		t.Fatal(err)
	}
	first := loadRepairReport(t, store, "report-repair-cas")
	second := loadRepairReport(t, store, first.ID)
	var raw string
	if err := store.DB().QueryRowContext(ctx, `SELECT report_json FROM ip_lookup_reports WHERE id=$1`, first.ID).Scan(&raw); err != nil ||
		first.RepairRevision != ipRepairRevision(raw) || len(first.RepairRevision) != 64 {
		t.Fatal("loader did not bind exact stored document", err)
	}
	first.Facts.City, second.Facts.City = "first-winner", "second-winner"
	var revisions [2]string
	errs := runTogether(func() error {
		var err error
		revisions[0], err = store.UpdateIPLookupCachedReport(ctx, first)
		return err
	}, func() error {
		var err error
		revisions[1], err = NewStore(store.DB()).UpdateIPLookupCachedReport(ctx, second)
		return err
	})
	winners, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			winners++
		} else if errors.Is(err, iplookup.ErrRepairConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	current := loadRepairReport(t, store, first.ID)
	expectedCity, expectedRevision := first.Facts.City, revisions[0]
	if expectedRevision == "" {
		expectedCity, expectedRevision = second.Facts.City, revisions[1]
	}
	if winners != 1 || conflicts != 1 || current.Facts.City != expectedCity ||
		current.RepairRevision != expectedRevision || current.RepairRevision == first.RepairRevision {
		t.Fatal("partial repair lost winning proof", winners, conflicts, current, revisions)
	}
	if _, err := store.UpdateIPLookupCachedReport(ctx, first); !errors.Is(err, iplookup.ErrRepairConflict) {
		t.Fatal("old worker overwrote committed partial repair", err)
	}
	current.Facts.Country = "NZ"
	nextRevision, err := store.UpdateIPLookupCachedReport(ctx, current)
	if err != nil || nextRevision == current.RepairRevision {
		t.Fatal("current worker could not advance its revision", nextRevision, err)
	}
	current.RepairRevision, current.Status = nextRevision, "succeeded"
	if _, err := store.UpdateIPLookupCachedReport(ctx, current); err != nil {
		t.Fatal("advanced worker could not finish report", err)
	}
	var at, status string
	if err := store.DB().QueryRowContext(ctx, `SELECT completed_at,status FROM ip_lookup_reports WHERE id=$1`, first.ID).Scan(&at, &status); err != nil || at != stamp(completed) || status != "succeeded" {
		t.Fatal("repair changed original cache ordering", at, status, err)
	}
}

func TestReportRepairCannotChangeFrozenIdentityOrPolicy(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	insertIPCache(t, store, "report-repair-identity", "8.8.8.8", "partial", time.Now().UTC())
	original := loadRepairReport(t, store, "report-repair-identity")
	for _, change := range []func(*iplookup.Report){
		func(r *iplookup.Report) { r.ID = "another-report" },
		func(r *iplookup.Report) { r.IP = "1.1.1.1" },
		func(r *iplookup.Report) { r.PolicyVersion += "-changed" },
		func(r *iplookup.Report) { r.ParserVersion += "-changed" },
		func(r *iplookup.Report) { r.Status = "processing" },
		func(r *iplookup.Report) { r.RepairRevision = "" },
	} {
		candidate := loadRepairReport(t, store, original.ID)
		change(&candidate)
		if _, err := store.UpdateIPLookupCachedReport(ctx, candidate); !errors.Is(err, iplookup.ErrRepairConflict) {
			t.Fatal("repair replaced frozen identity, policy or state", err)
		}
		if current := loadRepairReport(t, store, original.ID); current.RepairRevision != original.RepairRevision {
			t.Fatal("rejected repair changed stored document")
		}
	}
}
