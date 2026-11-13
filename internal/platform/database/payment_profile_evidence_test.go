package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPaymentProfileRetainsEveryOrderAccount(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"ezpay:retained-account:alipay", "ezpay:alipay"} {
		for _, status := range []string{"creating", "pending", "paid", "expired", "cancelled", "refunded", "failed"} {
			t.Run(method+"/"+status, func(t *testing.T) {
				t.Parallel()
				ctx, store := context.Background(), newTestStore(t)
				user := createTestUser(t, store, 69_040)
				input := PaymentProfileInput{ID: "retained-account", Provider: "ezpay", Endpoint: "https://account.example", MerchantID: "merchant", CredentialCiphertext: "old-ciphertext", EnabledChannels: []string{"alipay"}, Enabled: true}
				if _, err := store.SavePaymentProfile(ctx, input); err != nil {
					t.Fatal(err)
				}
				storedStatus := status
				if status == "cancelled" {
					storedStatus = "pending"
				}
				if _, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{ID: "retained-order", UserID: user.ID, Provider: "ezpay", MethodID: method, ProviderRail: "alipay", Status: storedStatus, TXBMinor: 100, PayableAmount: "1.00", PayableCurrency: "CNY", RateSnapshot: "1", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
					t.Fatal(err)
				}
				if status == "cancelled" {
					if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET cancelled_at=$1 WHERE id='retained-order'`, stamp(time.Now())); err != nil {
						t.Fatal(err)
					}
				}
				input.CredentialCiphertext = "replacement-ciphertext"
				if _, err := store.SavePaymentProfile(ctx, input); !errors.Is(err, ErrPaymentProfileInUse) {
					t.Fatalf("retained %s order lost callback credential: %v", status, err)
				}
				if err := store.DeletePaymentProfile(ctx, input.ID, ""); !errors.Is(err, ErrPaymentProfileInUse) {
					t.Fatalf("retained %s order account deleted: %v", status, err)
				}
				input.CredentialCiphertext, input.ProviderName, input.Enabled = "", "Retired account", false
				if _, err := store.SavePaymentProfile(ctx, input); err != nil {
					t.Fatal("disabling an old account must preserve callbacks", err)
				}
				stored, err := store.PaymentProfileRecordByID(ctx, input.ID, "")
				if err != nil || stored.Enabled || stored.CredentialCiphertext != "old-ciphertext" {
					t.Fatal("old account identity changed", err)
				}
			})
		}
	}
}

func TestPaymentProfileMutationRollsBackWhenAuditCannotCommit(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"save", "delete", "availability", "conditional availability"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			input := PaymentProfileInput{ID: "audited-account", Provider: "ezpay", ProviderName: "Before", Endpoint: "https://account.example", CredentialCiphertext: "ciphertext", Enabled: true}
			profile, err := store.SavePaymentProfile(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `CREATE FUNCTION reject_profile_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$`); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `CREATE TRIGGER reject_profile_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.target_type='payment_profile') EXECUTE FUNCTION reject_profile_audit()`); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "save":
				input.ProviderName, input.CredentialCiphertext = "After", ""
				_, err = store.SavePaymentProfile(ctx, input)
			case "delete":
				err = store.DeletePaymentProfile(ctx, input.ID, "")
			case "availability":
				_, err = store.SetPaymentProfileEnabled(ctx, input.ID, false)
			case "conditional availability":
				_, _, err = store.DisablePaymentProfileIfUnchanged(ctx, input.ID, profile.UpdatedAt)
			}
			if err == nil {
				t.Fatal("mutation ignored failed audit")
			}
			stored, err := store.PaymentProfileRecordByID(ctx, input.ID, "")
			if err != nil || stored.ProviderName != "Before" || !stored.Enabled || stored.CredentialCiphertext != "ciphertext" {
				t.Fatal("audit failure left committed profile mutation", err)
			}
		})
	}
}

func TestPaymentOrderAdmissionRequiresExistingPinnedAccount(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := lockPaymentOrderProfileTx(ctx, tx, model.PaymentOrder{Provider: "ezpay", MethodID: "ezpay:deleted-account:alipay"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing pinned account accepted: %v", err)
	}
}
