package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestUsageBreakdownHourlyUsesExactEnrollmentOwnerGenerationAndTuple(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	old := hourlyBreakdownEnrollment(t, s, u, "old-generation", "old-group", at.Add(-2*time.Hour))
	hourlyBreakdownReceipt(t, s, old, "old-receipt", 99, at)
	other := createTestUser(t, s, 91001)
	foreign := hourlyBreakdownEnrollment(t, s, other, "foreign", "foreign-group", at)
	hourlyBreakdownReceipt(t, s, foreign, "foreign-receipt", 999, at)
	usageBreakdownExec(t, s, `UPDATE users SET account_generation=3,created_at=$2 WHERE id=$1`, u.ID, stamp(at.Add(-time.Hour)))
	u.AccountGeneration = 3
	current := hourlyBreakdownEnrollment(t, s, u, "current", "retired-group-id", at)
	const exact = int64(9007199254740993)
	hourlyBreakdownReceipt(t, s, current, "current-receipt", exact, at)
	hourlyBreakdownRaw(t, s, current, "one", exact, 2, at)
	hourlyBreakdownRaw(t, s, current, "two", exact, 3, at)
	// All these malformed display rows pass the broad owner/window filter;
	// none may borrow another activation's attribution or financial identity.
	for n, entry := range []struct {
		id, period string
		principal  int64
	}{{foreign.EnrollmentID, foreign.PeriodID, foreign.ProxyUserID}, {old.EnrollmentID, old.PeriodID, old.ProxyUserID}, {current.EnrollmentID, current.PeriodID, current.ProxyUserID + 1}, {current.EnrollmentID, "wrong-period", current.ProxyUserID}, {"orphan", current.PeriodID, current.ProxyUserID}} {
		usageBreakdownExec(t, s, `INSERT INTO xnode_usage_minutes(owner_id,account_generation,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes)
 VALUES($1,3,'hourly',$2,$3,$4,$5,$6,$6,$6,999,999)`, u.ID, entry.id, entry.principal, entry.period, []string{"forged-owner", "forged-generation", "forged-principal", "forged-period", "orphan"}[n], at)
	}
	wrong := hourlyBreakdownEnrollment(t, s, u, "wrong-ledger", "wrong-ledger-group", at)
	usageBreakdownLedger(t, s, other.ID, "foreign-ledger", "hourly_charge", 333, at)
	usageBreakdownExec(t, s, `INSERT INTO hourly_settlements(id,enrollment_id,hour_start,charged_from,paid_until,hourly_price_minor,charged_minor,offer_revision,policy_version,ledger_id,created_at)
 VALUES('mismatched-ledger',$1,$2,$3,$4,333,333,1,$5,'foreign-ledger',$3)`, wrong.EnrollmentID, at.Truncate(time.Hour), at, at.Truncate(time.Hour).Add(time.Hour), strings.Repeat("b", 64))
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at)
	if err != nil || len(value.Nodes) != 0 || len(value.HourlySquads) != 1 {
		t.Fatal(value, err)
	}
	squad := value.HourlySquads[0]
	if squad.GroupID != current.GroupID || squad.Name != current.GroupID || squad.UploadBytes != "18014398509481986" || squad.DownloadBytes != "5" || squad.CostMinor.Numerator != "9007199254740993" || squad.CostMinor.Denominator != "1" {
		t.Fatal("scope leaked or exact values rounded", squad)
	}
	if value.Cost.ActualMinor != squad.CostMinor.Numerator || value.PositiveNodes.Traffic != "0" || value.PositiveEntries.Traffic != "1" || value.PositiveEntries.Cost != "1" {
		t.Fatal(value)
	}
	assertUsageBreakdownConservation(t, value)
}
