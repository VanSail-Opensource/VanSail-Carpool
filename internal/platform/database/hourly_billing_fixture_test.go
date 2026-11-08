package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func hourlyBillingFixture(t *testing.T) (*Store, model.HourlyPolicySnapshot, HourlyActivationInput) {
	t.Helper()
	s := newTestStore(t)
	u := createTestUser(t, s, 89710)
	ctx := context.Background()
	policy := model.DefaultHourlyPolicy()
	policy.SalesEnabled, policy.DailyAllowanceBytes = true, 1000
	policy.ReducedRateBytesPerSecond, policy.ExtraGBPriceTXBMinor = 1024, 25
	record, err := s.CompareAndSwapHourlyPolicy(ctx, u.ID, "", "opaque-billing-policy-fixture")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := model.HourlyPolicySnapshot{Version: record.Version, Policy: policy}
	input := HourlyActivationInput{OwnerID: u.ID, AccountGeneration: u.AccountGeneration, GroupID: "billing-hourly-a",
		ActivationKey: "activation-one", ExpectedPolicyVersion: record.Version, ExpectedOfferRevision: 1}
	addHourlyBillingOffer(t, s, u.ID, input.GroupID, 100)
	if _, err := s.AdjustBalance(ctx, u.ID, 100000, "hourly-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	return s, snapshot, input
}

func addHourlyBillingOffer(t *testing.T, s *Store, actor, group string, price int64) {
	t.Helper()
	ctx := context.Background()
	if err := s.SaveXNodeGroup(ctx, xnode.Group{ID: group, Name: group, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveHourlyOffer(ctx, actor, model.HourlyOffer{GroupID: group, Enabled: true, HourlyPriceTXBMinor: price}, 0); err != nil {
		t.Fatal(err)
	}
}

func prepareHourlyBillingFixture(t *testing.T, s *Store, input HourlyActivationInput, snapshot model.HourlyPolicySnapshot) HourlyCommitInput {
	t.Helper()
	e, err := s.PrepareHourlyActivation(context.Background(), input, snapshot, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.EnsureXNodeHourlyPrincipal(context.Background(), e.ID, "opaque-hourly-credentials")
	if err != nil {
		t.Fatal(err)
	}
	guard := hourlyBillingDelivery(t, s, e.ID)[0]
	return HourlyCommitInput{HourlyActivationInput: input, EnrollmentID: e.ID, ProxyUserID: p.ProxyUserID, PeriodID: p.PeriodID,
		ExpectedCompileRevision: guard.CompileRevision, Delivery: guard}
}

func activateHourlyBillingFixture(t *testing.T, s *Store, input HourlyActivationInput, snapshot model.HourlyPolicySnapshot) HourlyActivationResult {
	t.Helper()
	result, err := s.CommitHourlyActivation(context.Background(), prepareHourlyBillingFixture(t, s, input, snapshot), snapshot, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func hourlyBillingNow(t *testing.T, s *Store) time.Time {
	t.Helper()
	var at time.Time
	if err := s.DB().QueryRow(`SELECT clock_timestamp()`).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at.UTC()
}

func hourlyBillingBalance(t *testing.T, s *Store, owner string) int64 {
	t.Helper()
	b, err := s.Balance(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return b.MinorInt64()
}

func hourlyBillingDelivery(t *testing.T, s *Store, ids ...string) []HourlyDeliveryReady {
	t.Helper()
	var revision int64
	if err := s.DB().QueryRow(`SELECT revision FROM xnode_compile_revision WHERE singleton=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	now := hourlyBillingNow(t, s)
	result := make([]HourlyDeliveryReady, 0, len(ids))
	for _, id := range ids {
		result = append(result, HourlyDeliveryReady{EnrollmentID: id, ObservedAt: now, ReadyUntil: now.Add(4 * time.Second), CompileRevision: revision})
	}
	return result
}

func makeDueHourlyBillingFixture(t *testing.T, s *Store, input HourlyActivationInput, snapshot model.HourlyPolicySnapshot, activated time.Time) model.HourlyEnrollment {
	t.Helper()
	prepared := prepareHourlyBillingFixture(t, s, input, snapshot)
	// Construct a historical paid activation without charging this test's wallet;
	// the exercised transition begins only at its overdue current-hour renewal.
	until := hourlyBillingNow(t, s).Truncate(time.Hour).Add(-time.Hour)
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET status='active',activated_at=$1,paid_through=$2 WHERE id=$3`, activated, until, prepared.EnrollmentID); err != nil {
		t.Fatal(err)
	}
	e, err := s.HourlyEnrollment(context.Background(), prepared.EnrollmentID)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
