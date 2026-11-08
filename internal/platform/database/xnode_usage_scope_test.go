package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyHistoryValidatesOriginalSubjectAndRetainsOldGeneration(t *testing.T) {
	s, identity, period, p, at := usageHistoryFixture(t)
	ctx := context.Background()
	base := usageHistoryReport(identity, period, p, at, 1)
	base.Samples = nil // Accepted Begin history does not require writer success.
	original := base.HourlySamples[0]
	for _, mutate := range []func(*wire.HourlySample){
		func(v *wire.HourlySample) { v.Owner.ID = "other-owner" },
		func(v *wire.HourlySample) { v.Owner.Generation++ },
		func(v *wire.HourlySample) { v.UserID++ },
		func(v *wire.HourlySample) { v.AccountingPeriod = "other-period" },
		func(v *wire.HourlySample) { v.EnrollmentID = "other-enrollment" },
		func(v *wire.HourlySample) { v.DayUTC = at.AddDate(0, 0, -1).Format(time.DateOnly) },
		func(v *wire.HourlySample) { v.CounterGeneration = 0 },
		func(v *wire.HourlySample) { v.From = v.Through.Add(time.Second) },
	} {
		value := original
		mutate(&value)
		base.HourlySamples = []wire.HourlySample{value}
		if err := commitUsageHistoryFixture(t, s, base, false); !errors.Is(err, xnode.ErrInvalid) {
			t.Fatal("mismatched raw subject reached history", value, err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET account_generation=7 WHERE id=$1`, identity.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE hourly_enrollments SET status='disabled',disabled_at=clock_timestamp() WHERE id=$1`, p.EnrollmentID); err != nil {
		t.Fatal(err)
	}
	for sequence := uint64(1); sequence <= 2; sequence++ {
		report := usageHistoryReport(identity, period, p, at, sequence)
		report.Samples = nil
		if err := commitUsageHistoryFixture(t, s, report, false); err != nil {
			t.Fatal("original generation/closed-enrollment history was refused", err)
		}
	}
	rows, _, err := s.XNodeUsageHistory(ctx, identity.UserID, at.Add(-time.Minute), at.Add(time.Second))
	if err != nil || len(rows) != 1 || rows[0].UploadBytes != "14" || rows[0].DownloadBytes != "18" {
		t.Fatal("counter incarnation was mistaken for report deduplication", rows, err)
	}
	var generation int64
	if err = s.db.QueryRow(`SELECT account_generation FROM xnode_usage_minutes`).Scan(&generation); err != nil || generation != 0 {
		t.Fatal("late history was relabeled with current generation", generation, err)
	}
}

func TestHourlyHistoryPresentationDoesNotReauthorizeByServerPaidClock(t *testing.T) {
	s, identity, period, p, at := usageHistoryFixture(t)
	report := usageHistoryReport(identity, period, p, at, 1)
	report.Samples = nil
	remote := p.ActivatedAt.Add(-500 * time.Millisecond)
	report.HourlySamples[0].From, report.HourlySamples[0].Through = remote, remote
	report.HourlySamples[0].DayUTC = remote.Format(time.DateOnly)
	if err := commitUsageHistoryFixture(t, s, report, false); err != nil {
		t.Fatal("ordinary remote clock offset blocked the mixed journal", err)
	}
}

func TestHourlyHistoryRejectsAnUnregisteredNode(t *testing.T) {
	s, identity, period, p, at := usageHistoryFixture(t)
	ctx := context.Background()
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "foreign-node", ConfigID: "config", Name: "Foreign", Enabled: true, SettingsCipher: "encrypted", MultiplierFP: xnode.MultiplierScale}); err != nil {
		t.Fatal(err)
	}
	if err := s.BindXNodeAgent(ctx, "foreign-node", "foreign-agent"); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterXNodeTrafficStream(ctx, "foreign-node", "foreign-agent", usageHistoryStream); err != nil {
		t.Fatal(err)
	}
	report := usageHistoryReport(identity, period, p, at, 1)
	report.ID, report.Samples = "foreign-agent:"+usageHistoryStream+":1", nil
	if err := s.RecordXNodeTraffic(ctx, "foreign-node", "foreign-agent", 1, "foreign", report); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("unregistered participant attributed another node's admission", err)
	}
}
