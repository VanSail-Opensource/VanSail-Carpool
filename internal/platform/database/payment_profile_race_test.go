package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPaymentProfileMutationWaitsForAdmittedOrder(t *testing.T) {
	for _, mutation := range []string{"retarget", "delete"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			store := newTestStore(t)
			user := createTestUser(t, store, 69411)
			profile := PaymentProfileInput{ID: "locked-account", Provider: "ezpay", Endpoint: "https://original.example", MerchantID: "original", CredentialCiphertext: "original-secret", Enabled: true}
			if _, err := store.SavePaymentProfile(ctx, profile); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			order := model.PaymentOrder{ID: "admitted-order", UserID: user.ID, Provider: profile.Provider,
				MethodID: "ezpay:locked-account:alipay", ProviderRail: "alipay", Status: "creating", TXBMinor: 100,
				PayableAmount: "1.00", PayableCurrency: "CNY", RateSnapshot: "1", RateDirection: "currency_per_txb", ExpiresAt: now.Add(time.Hour)}
			if err := insertPaymentOrderTx(ctx, tx, order, now); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				if mutation == "delete" {
					done <- store.DeletePaymentProfile(ctx, profile.ID, "")
					return
				}
				changed := profile
				changed.Endpoint, changed.MerchantID, changed.CredentialCiphertext = "https://replacement.example", "replacement", "replacement-secret"
				_, err := store.SavePaymentProfile(ctx, changed)
				done <- err
			}()
			waitForPaymentProfileLock(t, ctx, store, "FOR UPDATE")
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if err := <-done; !errors.Is(err, ErrPaymentProfileInUse) {
				t.Fatalf("%s crossed committed order admission: %v", mutation, err)
			}
			retained, err := store.PaymentProfileRecordByID(ctx, profile.ID, "")
			if err != nil || retained.Endpoint != profile.Endpoint || retained.MerchantID != profile.MerchantID || retained.CredentialCiphertext != profile.CredentialCiphertext {
				t.Fatalf("admitted order lost its provider identity: %+v, %v", retained, err)
			}
		})
	}
}

func TestPaymentAdmissionWaitsForCommittedProfileDeletion(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := newTestStore(t)
	user := createTestUser(t, store, 69412)
	profile := PaymentProfileInput{ID: "removed-account", Provider: "ezpay", Endpoint: "https://original.example", CredentialCiphertext: "secret", Enabled: true}
	if _, err := store.SavePaymentProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM payment_profiles WHERE id=$1`, profile.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	order := model.PaymentOrder{ID: "rejected-order", UserID: user.ID, Provider: profile.Provider,
		MethodID: "ezpay:removed-account:alipay", ProviderRail: "alipay", Status: "creating", TXBMinor: 100,
		PayableAmount: "1.00", PayableCurrency: "CNY", RateSnapshot: "1", RateDirection: "currency_per_txb", ExpiresAt: now.Add(time.Hour)}
	done := make(chan error, 1)
	go func() {
		_, _, err := store.BeginPaymentOrderOperation(ctx, order, providerops.CreateInput{
			ActorUserID: user.ID, OwnerUserID: user.ID, Kind: "payment_create", IdempotencyKey: "removed-profile",
			RequestFingerprint: "removed-profile-fingerprint", Items: []providerops.ItemInput{{Key: "order", TargetType: "payment_order", TargetID: order.ID}},
		}, now)
		done <- err
	}()
	waitForPaymentProfileLock(t, ctx, store, "FOR SHARE")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrNotFound) {
		t.Fatalf("new order used a deleted pinned profile: %v", err)
	}
	var orders, operations, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM payment_orders WHERE id=$1),
		(SELECT count(*) FROM provider_operations WHERE actor_user_id=$2),
		(SELECT count(*) FROM outbox_jobs WHERE kind='provider_operation')`, order.ID, user.ID).
		Scan(&orders, &operations, &jobs); err != nil || orders != 0 || operations != 0 || jobs != 0 {
		t.Fatalf("failed admission left durable work: orders=%d operations=%d jobs=%d err=%v", orders, operations, jobs, err)
	}
}

func waitForPaymentProfileLock(t *testing.T, ctx context.Context, store *Store, clause string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
			WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'
			AND strpos(query,'payment_profiles')>0 AND strpos(query,$1)>0)`, clause).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("concurrent operation did not wait for the payment profile lock")
}
