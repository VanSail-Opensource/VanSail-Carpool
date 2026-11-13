package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestUsageBreakdownPreservesNanosecondLedgerBounds(t *testing.T) {
	t.Parallel()
	s, u, at := usageBreakdownFixture(t)
	charged := at.Add(time.Nanosecond)
	account := usageBreakdownPAYGAccount(t, s, u, "exact-payg", at.Add(-time.Hour))
	usageBreakdownPAYGCharge(t, s, account, "exact-payg-charge", 0, 1, 9, charged)
	usageBreakdownHourlyCharges(t, s, u, "exact-hourly", charged)
	for _, item := range []struct {
		name     string
		from, to time.Time
		actual   string
	}{
		{"before charge", at.Add(-time.Minute), at, "0"},
		{"inclusive instant", charged, charged, "45"},
		{"after charge", charged.Add(time.Nanosecond), at.Add(time.Minute), "0"},
		{"containing interval", at, charged.Add(time.Nanosecond), "45"},
	} {
		t.Run(item.name, func(t *testing.T) {
			value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, item.from, item.to)
			if err != nil || value.Cost.ActualMinor != item.actual {
				t.Fatalf("nanosecond receipt interval = %+v, %v; want %s", value.Cost, err, item.actual)
			}
			if item.actual == "45" && (len(value.Nodes) != 1 || len(value.HourlySquads) != 1 || value.Cost.UnallocatedMinor.Numerator != "25") {
				t.Fatalf("receipt kinds disagree at exact instant: %+v", value)
			}
			assertUsageBreakdownConservation(t, value)
		})
	}
}

func TestUsageBreakdownLegacyExpiryRetainsFinalNanosecond(t *testing.T) {
	t.Parallel()
	s, u, at := usageBreakdownFixture(t)
	seconds := int64(7 * 86400)
	usageBreakdownPurchase(t, s, u, "exact-legacy", 100, &seconds, at)
	until := at.Add(time.Nanosecond)
	usageBreakdownExec(t, s, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id='exact-legacy'`, stamp(at), stamp(until))
	for _, item := range []struct {
		at     time.Time
		status string
	}{{at, model.UsageLegacyAvailable}, {until, model.UsageLegacyNone}} {
		value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), item.at)
		if err != nil || value.Cost.Legacy.Status != item.status {
			t.Fatalf("legacy at %v = %+v, %v", item.at, value.Cost.Legacy, err)
		}
		assertUsageBreakdownConservation(t, value)
	}
}

func TestUsageBreakdownCoverageSeparatesExactPeriodsAndTypedObservationBounds(t *testing.T) {
	t.Parallel()
	s, u, at := usageBreakdownFixture(t)
	account := usageBreakdownPAYGAccount(t, s, u, "exact-coverage", at.Add(-time.Hour))
	usageBreakdownExec(t, s, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly)
 VALUES('exact-coverage','agent','stream',1,'digest','sample-capacity',$1,1,0)`, at)
	assertGaps := func(from, through time.Time, want string) {
		t.Helper()
		value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, from, through)
		if err != nil || value.Coverage.GapCount != want {
			t.Fatalf("coverage [%v,%v] = %+v, %v; want %s", from, through, value.Coverage, err, want)
		}
	}
	assertGaps(at.Add(time.Nanosecond), at.Add(time.Minute), "0")
	usageBreakdownExec(t, s, `UPDATE xnode_usage_coverage SET observed_at=$1`, at.Add(time.Microsecond))
	assertGaps(at, at.Add(time.Microsecond-time.Nanosecond), "0")
	assertGaps(at, at.Add(time.Microsecond), "1")
	usageBreakdownExec(t, s, `UPDATE xnode_periods SET status='closed',ended_at=$2 WHERE id=$1`, account.PeriodID, stamp(at.Add(time.Nanosecond)))
	assertGaps(at.Add(2*time.Nanosecond), at.Add(time.Minute), "0")
}

func TestUsageBreakdownTypedLifecycleDoesNotIncludeEarlierObservation(t *testing.T) {
	t.Parallel()
	s, u, at := usageBreakdownFixture(t)
	usageBreakdownExec(t, s, `UPDATE users SET created_at=$2 WHERE id=$1`, u.ID, stamp(at.Add(time.Nanosecond)))
	usageBreakdownRaw(t, s, u, "combo", "retained", "before-lifecycle", 100, 0, at)
	usageBreakdownRaw(t, s, u, "combo", "retained", "current-lifecycle", 7, 0, at.Add(time.Microsecond))
	principal := hourlyBreakdownEnrollment(t, s, u, "lifecycle-hourly", "lifecycle-group", at)
	hourlyBreakdownRaw(t, s, principal, "before-lifecycle", 200, 0, at)
	hourlyBreakdownRaw(t, s, principal, "current-lifecycle", 11, 0, at.Add(time.Microsecond))
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-time.Minute), at.Add(time.Minute))
	if err != nil || len(value.Nodes) != 1 || value.Nodes[0].UploadBytes != "7" ||
		len(value.HourlySquads) != 1 || value.HourlySquads[0].UploadBytes != "11" {
		t.Fatalf("earlier observation crossed exact lifecycle: %+v, %v", value, err)
	}
}
