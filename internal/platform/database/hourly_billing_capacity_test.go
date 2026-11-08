package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestHourlyGlobalCapacityPreservesExistingAndOverdueOwners(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	other := createTestUser(t, s, 89714)
	ctx := context.Background()
	check := func(owner string, generation int64) error {
		_, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (struct{}, error) {
			if err := lockHourlyOwnerTx(ctx, tx, owner, generation); err != nil {
				return struct{}{}, err
			}
			if _, err := balanceTx(ctx, tx, owner); err != nil {
				return struct{}{}, err
			}
			if _, err := hourlyCompileRevisionTx(ctx, tx); err != nil {
				return struct{}{}, err
			}
			// Use one slot to exercise the same production boundary cheaply.
			return struct{}{}, requireHourlyOwnerCapacityTx(ctx, tx, owner, generation, 1)
		})
		return err
	}
	if err := check(other.ID, other.AccountGeneration); !errors.Is(err, ErrHourlyCapacity) {
		t.Fatalf("overdue but ongoing owner lost its reserved place: %v", err)
	}
	if err := check(input.OwnerID, input.AccountGeneration); err != nil {
		t.Fatalf("existing owner could not add a squad at capacity: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	if err := check(other.ID, other.AccountGeneration); err != nil {
		t.Fatalf("stale historical generation occupied a current capacity slot: %v", err)
	}
}
