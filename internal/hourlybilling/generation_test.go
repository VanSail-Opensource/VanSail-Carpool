package hourlybilling

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestSupersededHourlyCommandsTerminateWithoutCurrentGenerationMutation(t *testing.T) {
	for _, phase := range []string{"queued", "processing", "purging"} {
		t.Run(phase, func(t *testing.T) {
			s, user, policy, native := commerceFixture(t)
			op, job := queuedActivation(t, s, user, policy)
			if phase != "queued" {
				var err error
				op, err = s.store.BeginProviderOperationAttempt(context.Background(), op.Receipt.ID, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.store.BeginProviderOperationItemAttempt(context.Background(), op.Receipt.ID, "hourly", time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			query := `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`
			if phase == "purging" {
				query = `UPDATE users SET account_status='purging' WHERE id=$1`
			}
			if _, err := s.store.DB().Exec(query, user.ID); err != nil {
				t.Fatal(err)
			}
			if err := s.HandleProviderOperation(context.Background(), op, job); err != nil {
				t.Fatal(err)
			}
			op, _ = reloadOperation(t, s, op.Receipt.ID)
			if op.Receipt.Status != "failed" || op.Receipt.ErrorCode == nil || *op.Receipt.ErrorCode != "ACCOUNT_GENERATION_SUPERSEDED" {
				t.Fatal("superseded receipt remained pending or lost its cause")
			}
			items, err := s.store.ProviderOperationItems(context.Background(), op.Receipt.ID)
			if err != nil || len(items) != 1 || items[0].Status != providerops.StatusFailed {
				t.Fatal("superseded item did not complete its legal phase transition")
			}
			if native.resumes != 0 || native.closes != 0 || policy.reads != 0 || balanceFixture(t, s, user) != 100000 {
				t.Fatal("stale command touched current benefits, policy or wallet")
			}
		})
	}
}
