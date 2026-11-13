package database

import (
	"errors"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAdminResolutionRejectsUnsettledAndForeignDomainCommands(t *testing.T) {
	for _, kind := range []string{"payment_create", providerops.KindPaymentRefund, "purchase_refund", "hourly_activation",
		"hourly_extra", providerops.KindEmbySetup, "xnode_command", "xnode_routing", providerops.KindSubscriptionRevoke,
		providerops.KindAdminTemporaryBan, providerops.KindAdminTemporaryUnban} {
		t.Run(kind, func(t *testing.T) {
			store, input, now := resolutionFixture(t, kind, providerops.StatusSucceeded)
			if _, err := store.ResolveAdminOperation(t.Context(), input, now); !errors.Is(err, ErrOperationReconciliationRequired) || !errors.Is(err, ErrConflict) {
				t.Fatal("generic admin receipt repair accepted a domain command", err)
			}
			assertNoResolutionWrites(t, store, input.OperationID)
		})
	}
}

func TestAdminResolutionRequiresCompleteMatchingItemEvidence(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		outcomes []providerops.Status
	}{
		{"missing items", nil},
		{"queued", []providerops.Status{providerops.StatusQueued}},
		{"processing", []providerops.Status{providerops.StatusProcessing}},
		{"ambiguous", []providerops.Status{providerops.StatusPendingReview}},
		{"contradicted", []providerops.Status{providerops.StatusFailed}},
		{"mixed", []providerops.Status{providerops.StatusSucceeded, providerops.StatusFailed}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store, input, now := resolutionFixture(t, providerops.KindAdminBulkExtension, scenario.outcomes...)
			before, err := store.ProviderOperationItems(t.Context(), input.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.ResolveAdminOperation(t.Context(), input, now); !errors.Is(err, ErrOperationReconciliationRequired) {
				t.Fatal("receipt success was fabricated from insufficient evidence", err)
			}
			after, err := store.ProviderOperationItems(t.Context(), input.OperationID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("repair rewrote immutable item evidence", err)
			}
			assertNoResolutionWrites(t, store, input.OperationID)
		})
	}
}

func TestAdminResolutionRepairsOnlySupportedExactStateReceipts(t *testing.T) {
	for _, kind := range []string{providerops.KindAdminEntitlementEdit, providerops.KindAdminEntitlementRefund,
		providerops.KindAdminComboReplacement, providerops.KindAdminBulkExtension, providerops.KindLegacyOutageSync} {
		t.Run(kind, func(t *testing.T) {
			store, input, now := resolutionFixture(t, kind, providerops.StatusSucceeded)
			before, err := store.ProviderOperationItems(t.Context(), input.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := store.ResolveAdminOperation(t.Context(), input, now)
			if err != nil || receipt.Status != "succeeded" || receipt.ErrorCode != nil {
				t.Fatal("substantiated parent repair failed or retained stale error", receipt, err)
			}
			after, err := store.ProviderOperationItems(t.Context(), input.OperationID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("parent repair rewrote item outcomes", err)
			}
			operation, err := store.ProviderOperationByID(t.Context(), input.OperationID)
			if err != nil || operation.ResultJSON != `{"retained":9007199254740993}` {
				t.Fatal("parent repair changed original result evidence", err)
			}
			var ledger int
			if err := store.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM ledger_entries`).Scan(&ledger); err != nil || ledger != 0 {
				t.Fatal("receipt repair moved money", ledger, err)
			}
		})
	}
}

func TestAdminResolutionRechecksActorBeforeEvenReplaying(t *testing.T) {
	store, input, now := resolutionFixture(t, providerops.KindAdminEntitlementEdit, providerops.StatusSucceeded)
	if _, err := store.ResolveAdminOperation(t.Context(), input, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(t.Context(), `UPDATE users SET role='user' WHERE id=$1`, input.ActorUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveAdminOperation(t.Context(), input, now); !errors.Is(err, ErrConflict) {
		t.Fatal("demoted actor replayed privileged resolution", err)
	}
}
