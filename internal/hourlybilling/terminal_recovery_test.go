package hourlybilling

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestTerminalHourlyItemRecoversBeforeMutableTargetReads(t *testing.T) {
	for _, status := range []providerops.Status{providerops.StatusSucceeded, providerops.StatusFailed,
		providerops.StatusCompensated, providerops.StatusPendingReview} {
		t.Run(string(status), func(t *testing.T) {
			s, user, policy, native := commerceFixture(t)
			op, job := queuedActivation(t, s, user, policy)
			ctx := context.Background()
			var err error
			op, err = s.store.BeginProviderOperationAttempt(ctx, op.Receipt.ID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.store.BeginProviderOperationItemAttempt(ctx, op.Receipt.ID, "hourly", time.Now()); err != nil {
				t.Fatal(err)
			}
			completion := providerops.Completion{Status: status, ErrorCode: "RECORDED_OUTCOME",
				ProviderReference: "native-receipt", ResultJSON: `{"generation":9007199254740993}`}
			item, err := s.store.CompleteProviderOperationItem(ctx, op.Receipt.ID, "hourly", completion, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			// Model a crash after item completion and before the parent write.
			// Recovery must use the recorded outcome even if the encrypted input
			// is no longer readable and the owner has since changed generation.
			s.vault = nil
			job.Payload = "unavailable-target"
			if _, err = s.store.DB().ExecContext(ctx, `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, user.ID); err != nil {
				t.Fatal(err)
			}
			if err = s.HandleProviderOperation(ctx, op, job); err != nil {
				t.Fatal(err)
			}
			recovered, err := s.store.ProviderOperationByID(ctx, op.Receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Receipt.Status != string(status) || recovered.ProviderReference != item.ProviderReference ||
				recovered.ResultJSON != item.ResultJSON || recovered.Receipt.ErrorCode == nil || *recovered.Receipt.ErrorCode != item.ErrorCode {
				t.Fatalf("recorded result changed: %+v", recovered)
			}
			if policy.reads != 0 || native.resumes != 0 || native.closes != 0 || balanceFixture(t, s, user) != 100000 {
				t.Fatal("terminal recovery revisited policy, native access or the wallet")
			}
		})
	}
}
