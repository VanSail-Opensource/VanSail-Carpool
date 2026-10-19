package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestNativeOnlyResetRetainsPurchaseWideRefundEvidence(t *testing.T) {
	f := newNativePurchaseFixture(t)
	ctx := context.Background()
	identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=7 WHERE id=?`, identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	receipt := f.resetReceipt(t)
	operation, err := f.store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	worker := purchaseops.NewWorker(f.store, f.provider)
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	operation, err = f.store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil || operation.Receipt.Status != "succeeded" {
		t.Fatal(operation.Receipt, err)
	}
	identity, err = f.provider.native.IdentityForUser(ctx, f.user.ID)
	if err != nil || identity.CurrentPeriodID != receipt.ID {
		t.Fatal("reset lost its durable operation reference", identity, err)
	}
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 2}); err != nil {
		t.Fatal(err)
	}
	balance, err := f.store.Balance(ctx, f.user.ID)
	if err != nil || balance.Minor != "9690" {
		t.Fatal("reset charged more than once", balance, err)
	}
	quote, err := purchaseops.NewService(f.store, f.provider).RefundQuote(ctx, f.user.ID, f.purchase.ID)
	if err != nil || quote.Eligible || quote.ReasonCode == nil || *quote.ReasonCode != purchaseops.ReasonTrafficUsed {
		t.Fatal("reset erased purchase refund evidence", quote, err)
	}
	user, err := f.store.UserByID(ctx, f.user.ID)
	if err != nil || user.RemnaUserID != nil {
		t.Fatal("native reset required a legacy link", user, err)
	}
}

func TestNativeOnlyUnusedPurchaseRefund(t *testing.T) {
	f := newNativePurchaseFixture(t)
	ctx := context.Background()
	service := purchaseops.NewService(f.store, f.provider)
	receipt, err := service.RefundPurchase(ctx, f.user.ID, f.purchase.ID, "refund")
	if err != nil {
		t.Fatal(err)
	}
	operation, err := f.store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	worker := purchaseops.NewWorker(f.store, f.provider)
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	operation, err = f.store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil || operation.Receipt.Status != "succeeded" {
		t.Fatal(operation.Receipt, err)
	}
	balance, err := f.store.Balance(ctx, f.user.ID)
	if err != nil || balance.Minor != "10000" {
		t.Fatal(balance, err)
	}
}

type unavailablePurchaseRemote struct{ purchaseops.Remote }

func (unavailablePurchaseRemote) PurchaseRemote(context.Context, model.User, model.Purchase) (purchaseops.Remote, string, error) {
	return nil, "", errors.New("temporary identity read failure")
}

func TestExhaustedNativeResetAdmissionPreservesAttemptCertainty(t *testing.T) {
	for _, attempted := range []bool{false, true} {
		f := newNativePurchaseFixture(t)
		ctx := context.Background()
		receipt := f.resetReceipt(t)
		operation, err := f.store.ProviderOperationByID(ctx, receipt.ID)
		if err != nil {
			t.Fatal(err)
		}
		if attempted {
			operation, err = f.store.BeginProviderOperationAttempt(ctx, receipt.ID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.BeginProviderOperationItemAttempt(ctx, receipt.ID, "purchase", time.Now()); err != nil {
				t.Fatal(err)
			}
		}
		worker := purchaseops.NewWorker(f.store, unavailablePurchaseRemote{})
		if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 10}); err != nil {
			t.Fatal(err)
		}
		operation, err = f.store.ProviderOperationByID(ctx, receipt.ID)
		if err != nil {
			t.Fatal(err)
		}
		balance, err := f.store.Balance(ctx, f.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if attempted {
			if operation.Receipt.Status != "pending_review" || balance.Minor != "9690" {
				t.Fatal("uncertain reset was compensated", operation.Receipt, balance)
			}
		} else if operation.Receipt.Status != "compensated" || balance.Minor != "9700" {
			t.Fatal("unattempted reset was not compensated", operation.Receipt, balance)
		}
	}
}
