package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUsageBreakdownHourlyMinimumRecognizesElapsedCoverage(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	start := at.Truncate(time.Hour).Add(15 * time.Minute)
	const hours = 168
	p := hourlyBreakdownEnrollment(t, s, u, "minimum-enrollment", "minimum-squad", start)
	paidUntil := start.Add(hours * time.Hour)
	usageBreakdownMarkedHourlyReceipt(t, s, p, "minimum-charge", start, start, paidUntil, hours, 101, hours*101)
	disabled := start.Add(3*time.Hour + 30*time.Minute)
	usageBreakdownExec(t, s, `UPDATE hourly_enrollments SET status='disabled',activated_at=$2,paid_through=$3,disabled_at=$4 WHERE id=$1`, p.EnrollmentID, start, paidUntil, disabled)

	for _, tc := range []struct {
		name     string
		from, to time.Time
		want     string
	}{
		{"only elapsed coverage before cutoff", start, start.Add(90 * time.Minute), "151"},
		{"cutoff excludes unused prepayment", start, disabled.Add(time.Hour), "353"},
		{"receipt predates rolling window", start.Add(time.Hour), start.Add(73 * time.Hour), "252"},
		{"no recognition after disable", disabled, disabled.Add(time.Hour), "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertUsageBreakdownHourlyWindow(t, s, u, "minimum-squad", tc.from, tc.to, tc.want)
		})
	}
	var debit, amount int64
	if err := s.DB().QueryRowContext(context.Background(), `SELECT l.delta_txb_minor,s.charged_minor
 FROM ledger_entries l JOIN hourly_settlements s ON s.ledger_id=l.id WHERE l.id='minimum-charge'`).Scan(&debit, &amount); err != nil || debit != -hours*101 || amount != hours*101 {
		t.Fatalf("projection changed immutable prepayment: debit=%d, charged=%d, error=%v", debit, amount, err)
	}
}

func TestUsageBreakdownHourlyMeteredRenewalUsesCoverageNotPrefetchTime(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	start := at.Truncate(time.Hour).Add(30 * time.Minute)
	debit := start.Add(-time.Second)
	paidUntil := start.Add(30 * time.Minute)
	p := hourlyBreakdownEnrollment(t, s, u, "metered-enrollment", "metered-squad", start.Add(-2*time.Hour))
	usageBreakdownMarkedHourlyReceipt(t, s, p, "metered-charge", debit, start, paidUntil, 0, 61, 31)
	usageBreakdownExec(t, s, `UPDATE hourly_enrollments SET status='active',activated_at=$2,paid_through=$3 WHERE id=$1`, p.EnrollmentID, start.Add(-2*time.Hour), paidUntil)
	cut := start.Add(10 * time.Minute)

	for _, tc := range []struct {
		name     string
		from, to time.Time
		want     string
	}{
		{"prefetched but not yet used", debit.Add(-time.Second), start, "0"},
		{"elapsed prefix", start, cut, "10"},
		{"adjacent suffix keeps rounding carry", cut, paidUntil, "21"},
		{"whole interval spends exactly the original debit", start, paidUntil.Add(time.Minute), "31"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertUsageBreakdownHourlyWindow(t, s, u, "metered-squad", tc.from, tc.to, tc.want)
		})
	}
}

func TestUsageBreakdownHourlyUnmarkedReceiptRetainsExactLedgerBounds(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	chargedFrom := at.Truncate(time.Hour).Add(time.Hour)
	debit := chargedFrom.Add(-333 * time.Nanosecond)
	p := hourlyBreakdownEnrollment(t, s, u, "legacy-enrollment", "legacy-squad", at)
	usageBreakdownLedger(t, s, u.ID, "legacy-charge", "hourly_charge", 37, debit)
	usageBreakdownExec(t, s, `INSERT INTO hourly_settlements(id,enrollment_id,hour_start,charged_from,paid_until,hourly_price_minor,charged_minor,offer_revision,policy_version,ledger_id,created_at)
 VALUES('legacy-charge',$1,$2,$2,$3,37,37,1,$4,'legacy-charge',$2)`, p.EnrollmentID, chargedFrom, chargedFrom.Add(time.Hour), strings.Repeat("b", 64))

	for _, tc := range []struct {
		name     string
		from, to time.Time
		want     string
	}{
		{"last nanosecond before debit excludes it", debit.Add(-time.Second), debit.Add(-time.Nanosecond), "0"},
		{"debit instant is inclusive", debit, debit, "37"},
		{"paid interval does not rebook legacy debit", chargedFrom, chargedFrom.Add(time.Hour), "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertUsageBreakdownHourlyWindow(t, s, u, "legacy-squad", tc.from, tc.to, tc.want)
		})
	}
}

func usageBreakdownMarkedHourlyReceipt(t *testing.T, s *Store, p xnode.HourlyPrincipal, id string, debit, from, until time.Time, minimumHours int, price, amount int64) {
	t.Helper()
	usageBreakdownLedger(t, s, p.OwnerID, id, "hourly_charge", amount, debit)
	usageBreakdownExec(t, s, `INSERT INTO hourly_settlements(id,enrollment_id,hour_start,charged_from,paid_until,hourly_price_minor,charged_minor,offer_revision,policy_version,ledger_id,created_at,minimum_hours)
 VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$1,$9,$10)`, id, p.EnrollmentID, from.Truncate(time.Hour), from, until, price, amount, strings.Repeat("b", 64), debit, minimumHours)
}

func assertUsageBreakdownHourlyWindow(t *testing.T, s *Store, u model.User, group string, from, to time.Time, want string) {
	t.Helper()
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if value.Cost.ActualMinor != want || value.Cost.UnallocatedMinor.Numerator != "0" {
		t.Fatalf("wrong hourly cost for %s through %s: %+v; want %s attributed", from, to, value.Cost, want)
	}
	if want != "0" {
		squad := hourlyBreakdownSquad(t, value, group)
		if squad.CostMinor != (model.RationalMinor{Numerator: want, Denominator: "1"}) || squad.UploadBytes != "0" || squad.DownloadBytes != "0" {
			t.Fatalf("wrong cost-only hourly squad: %+v", squad)
		}
	} else if len(value.HourlySquads) != 0 {
		t.Fatalf("zero-cost interval must not invent traffic or costs: %+v", value.HourlySquads)
	}
	assertUsageBreakdownConservation(t, value)
}
