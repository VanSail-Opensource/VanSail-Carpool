package database

import (
	"context"
	"testing"
)

func TestHourlyPaymentReceiptsAndActivationTimeCannotBeRewritten(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	active := activateHourlyBillingFixture(t, s, input, policy)
	ctx := context.Background()
	for _, query := range []string{
		`UPDATE hourly_settlements SET charged_minor=charged_minor+1 WHERE id=$1`,
		`DELETE FROM hourly_settlements WHERE id=$1`,
	} {
		if _, err := s.DB().ExecContext(ctx, query, active.Settlement.ID); err == nil {
			t.Fatalf("accepted immutable receipt mutation: %s", query)
		}
	}
	before := hourlyBillingBalance(t, s, input.OwnerID)
	if _, err := s.DB().ExecContext(ctx, `UPDATE hourly_enrollments SET activated_at=activated_at+INTERVAL '1 second' WHERE id=$1`, active.Enrollment.ID); err == nil {
		t.Fatal("accepted activation-time rewrite")
	}
	if hourlyBillingBalance(t, s, input.OwnerID) != before {
		t.Fatal("rejected receipt mutation changed wallet")
	}
}
