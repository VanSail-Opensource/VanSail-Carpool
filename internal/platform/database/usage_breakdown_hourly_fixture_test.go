package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func hourlyBreakdownEnrollment(t *testing.T, s *Store, u model.User, id, group string, at time.Time) xnode.HourlyPrincipal {
	t.Helper()
	usageBreakdownExec(t, s, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at)
 VALUES($1,$2,$3,$4,$1,$5,'preparing',$6,$6)`, id, u.ID, u.AccountGeneration, group, strings.Repeat("b", 64), at)
	p, err := s.EnsureXNodeHourlyPrincipal(context.Background(), id, "opaque")
	if err != nil {
		t.Fatal(err)
	}
	usageBreakdownExec(t, s, `UPDATE xnode_periods SET started_at=$2 WHERE id=$1`, p.PeriodID, stamp(at.Add(-time.Hour)))
	return p
}

func hourlyBreakdownRaw(t *testing.T, s *Store, p xnode.HourlyPrincipal, node string, up, down int64, at time.Time) {
	t.Helper()
	usageBreakdownExec(t, s, `INSERT INTO xnode_usage_minutes(owner_id,account_generation,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes)
 VALUES($1,$2,'hourly',$3,$4,$5,$6,$7,$8,$8,$9,$10)`, p.OwnerID, p.AccountGeneration, p.EnrollmentID, p.ProxyUserID, p.PeriodID, node, at.Truncate(time.Minute), at, up, down)
}

func hourlyBreakdownReceipt(t *testing.T, s *Store, p xnode.HourlyPrincipal, id string, amount int64, at time.Time) {
	t.Helper()
	usageBreakdownLedger(t, s, p.OwnerID, id, "hourly_charge", amount, at)
	usageBreakdownExec(t, s, `INSERT INTO hourly_settlements(id,enrollment_id,hour_start,charged_from,paid_until,hourly_price_minor,charged_minor,offer_revision,policy_version,ledger_id,created_at)
 VALUES($1,$2,$3,$4,$5,$6,$6,1,$7,$1,$4)`, id, p.EnrollmentID, at.Truncate(time.Hour), at, at.Truncate(time.Hour).Add(time.Hour), amount, strings.Repeat("b", 64))
}

func hourlyBreakdownSquad(t *testing.T, value model.UsageBreakdown, id string) model.UsageBreakdownHourlySquad {
	t.Helper()
	for _, squad := range value.HourlySquads {
		if squad.GroupID == id {
			return squad
		}
	}
	t.Fatal("missing hourly squad", id)
	return model.UsageBreakdownHourlySquad{}
}
