package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func usageBreakdownHourlyCharges(t *testing.T, s *Store, u model.User, id string, at time.Time) {
	t.Helper()
	ctx := context.Background()
	usageBreakdownExec(t, s, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at)
 VALUES($1,$2,$3,$1,$1,$4,'preparing',$5,$5)`, id, u.ID, u.AccountGeneration, strings.Repeat("a", 64), at)
	p, err := s.EnsureXNodeHourlyPrincipal(ctx, id, "opaque")
	if err != nil {
		t.Fatal(err)
	}
	usageBreakdownLedger(t, s, u.ID, id+"-hour", "hourly_charge", 11, at)
	usageBreakdownExec(t, s, `INSERT INTO hourly_settlements(id,enrollment_id,hour_start,charged_from,paid_until,hourly_price_minor,charged_minor,offer_revision,policy_version,ledger_id,created_at)
 VALUES($1,$2,$3,$4,$5,11,11,1,$6,$1,$4)`, id+"-hour", id, at.Truncate(time.Hour), at, at.Truncate(time.Hour).Add(time.Hour), strings.Repeat("a", 64))
	usageBreakdownExec(t, s, `UPDATE hourly_enrollments SET status='active',activated_at=$2,paid_through=$3 WHERE id=$1`, id, at, at.Truncate(time.Hour).Add(time.Hour))
	if p.OwnerID != u.ID {
		t.Fatal("hourly owner fixture mismatch")
	}
	usageBreakdownExec(t, s, `INSERT INTO hourly_credit_days(owner_id,account_generation,day_utc,free_allowance_bytes,extra_funded_bytes,created_at,updated_at)
 VALUES($1,$2,$3,0,1000000000,$4,$4)`, u.ID, u.AccountGeneration, at.Format(time.DateOnly), stamp(at))
	usageBreakdownLedger(t, s, u.ID, id+"-extra", "hourly_extra_gb", 25, at)
	usageBreakdownExec(t, s, `INSERT INTO hourly_extra_purchases(id,owner_id,account_generation,day_utc,block_index,byte_count,unit_price_minor,policy_version,ledger_id,created_at)
 VALUES($1,$2,$3,$4,1,1000000000,25,$5,$1,$6)`, id+"-extra", u.ID, u.AccountGeneration, at.Format(time.DateOnly), strings.Repeat("a", 64), at)
}

func TestUsageBreakdownHourlyChargeBelongsToSquadAndSharedExtraRemainsUnallocated(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	usageBreakdownHourlyCharges(t, s, u, "paid-hourly", at)
	usageBreakdownRaw(t, s, u, "hourly", "paid-hourly", "first", 10, 20, at)
	usageBreakdownRaw(t, s, u, "hourly", "paid-hourly", "second", 40, 50, at)
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at.Add(time.Minute))
	if err != nil || value.Cost.ActualMinor != "36" || value.Cost.UnallocatedMinor != (model.RationalMinor{Numerator: "25", Denominator: "1"}) {
		t.Fatal(value, err)
	}
	if len(value.Nodes) != 0 || len(value.HourlySquads) != 1 || value.PositiveNodes.Traffic != "0" || value.PositiveNodes.Cost != "0" || value.PositiveEntries.Traffic != "1" || value.PositiveEntries.Cost != "1" {
		t.Fatal(value)
	}
	squad := value.HourlySquads[0]
	if squad.GroupID != "paid-hourly" || squad.UploadBytes != "50" || squad.DownloadBytes != "70" || squad.CostMinor != (model.RationalMinor{Numerator: "11", Denominator: "1"}) {
		t.Fatal("hourly receipt or multi-node traffic lost squad attribution", squad)
	}
	assertUsageBreakdownConservation(t, value)
}

func TestUsageBreakdownRejoinScopesFinancialAndRawHistory(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	usageBreakdownHourlyCharges(t, s, u, "old-hourly", at)
	a := usageBreakdownPAYGAccount(t, s, u, "old-payg", at)
	usageBreakdownPAYGCharge(t, s, a, "old-cost", 0, 1, 999, at)
	usageBreakdownRaw(t, s, u, "hourly", "old-hourly", "old-hourly-node", 900, 0, at)
	usageBreakdownRaw(t, s, u, "combo", "old-combo", "old-combo-node", 800, 0, at)
	lifecycle := at.Add(2*time.Minute + 20*time.Second)
	usageBreakdownExec(t, s, `UPDATE users SET account_generation=3,created_at=$2 WHERE id=$1`, u.ID, stamp(lifecycle))
	u.AccountGeneration = 3
	usageBreakdownHourlyCharges(t, s, u, "new-hourly", lifecycle.Add(time.Minute))
	usageBreakdownRaw(t, s, u, "hourly", "new-hourly", "new-node", 7, 8, lifecycle.Add(time.Minute))
	// Even a late old-generation sample with a recent timestamp cannot enter
	// the new hourly lifecycle; its immutable debit receipt remains retained.
	old := u
	old.AccountGeneration = 0
	usageBreakdownRaw(t, s, old, "hourly", "old-hourly", "old-late", 100, 0, lifecycle.Add(time.Minute))
	through := lifecycle.Add(2 * time.Minute)
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, through.Add(-72*time.Hour), through)
	if err != nil || len(value.Nodes) != 0 || len(value.HourlySquads) != 1 || value.HourlySquads[0].GroupID != "new-hourly" || value.HourlySquads[0].UploadBytes != "7" || value.HourlySquads[0].DownloadBytes != "8" || value.Cost.ActualMinor != "36" || value.Coverage.Complete {
		t.Fatal(value, err)
	}
	var receipts int
	if err = s.DB().QueryRow(`SELECT COUNT(*) FROM ledger_entries WHERE kind IN('hourly_charge','hourly_extra_gb','payg_gb')`).Scan(&receipts); err != nil || receipts != 5 {
		t.Fatal("projection erased original financial evidence", receipts, err)
	}
	assertUsageBreakdownConservation(t, value)
}
