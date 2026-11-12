package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestStarsHistoryRefundRequiresRetainedChargeAndPreservesUnmatchedProof(t *testing.T) {
	for _, test := range []struct {
		name, status, charge string
		validateErr          error
		wantReview           bool
	}{
		{name: "paid exact charge", status: "paid", charge: "charge"},
		{name: "already refunded exact charge", status: "refunded", charge: "charge"},
		{name: "another charge", status: "paid", charge: "other-charge", wantReview: true},
		{name: "missing retained charge", status: "paid", wantReview: true},
		{name: "unsettled payment", status: "pending", charge: "charge", wantReview: true},
		{name: "historically purged payment", validateErr: database.ErrNotFound, wantReview: true},
		{name: "amount or owner mismatch", validateErr: database.ErrConflict, wantReview: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &starsMemoryRepository{}
			order := model.PaymentOrder{ID: "order", Status: test.status}
			if test.charge != "" {
				order.ProviderTradeID = &test.charge
			}
			service := &starsMemoryBilling{order: order, validateErr: test.validateErr}
			worker := &starsReconciler{botID: 123, repository: repository, billing: service, now: time.Now}
			transaction := starRefundTransaction()
			if err := worker.applyTransaction(context.Background(), transaction); err != nil {
				t.Fatal(err)
			}
			if test.wantReview {
				if len(repository.refunds) != 0 || len(repository.resolved) != 0 || len(repository.reviews) != 1 {
					t.Fatalf("unproven refund applied or lost: %+v", repository)
				}
				proof := repository.reviews[0]
				if proof.BotID != 123 || proof.OrderID != "order" || proof.ChargeID != "charge" || proof.TelegramID != 42 || proof.PayableAmount != "25" || proof.ProofHash != starTransactionHash(transaction) {
					t.Fatalf("unmatched refund evidence = %+v", proof)
				}
			} else if len(repository.refunds) != 1 || len(repository.resolved) != 1 || repository.resolved[0] != "order:charge" || len(repository.reviews) != 0 {
				t.Fatalf("authoritative refund not resolved: %+v", repository)
			}
		})
	}
}

func TestStarsHistoryFinancialFailuresKeepPageRecoverable(t *testing.T) {
	for _, failure := range []string{"validate", "refund", "resolve", "review"} {
		charge, err := "charge", errors.New("storage interrupted")
		repository := &starsMemoryRepository{cursor: database.TelegramStarCursor{BotID: 123, Generation: 1}}
		service := &starsMemoryBilling{order: model.PaymentOrder{ID: "order", ProviderTradeID: &charge, Status: "paid"}}
		switch failure {
		case "validate":
			service.validateErr = err
		case "refund":
			repository.refundErr = err
		case "resolve":
			repository.resolveErr = err
		case "review":
			service.validateErr, repository.reviewErr = database.ErrNotFound, err
		}
		provider := starsHistoryTelegramForTest(t, func(int, int) ([]telegram.StarTransaction, error) {
			return []telegram.StarTransaction{starRefundTransaction()}, nil
		})
		worker := &starsReconciler{botID: 123, provider: provider, repository: repository, billing: service, now: time.Now}
		if err := worker.HandleOutbox(context.Background(), starsPageJob(t, repository.cursor)); err == nil || repository.advancements != 0 {
			t.Fatalf("%s failure discarded page: advanced=%d err=%v", failure, repository.advancements, err)
		}
	}
}

func TestStarsHistoryProofHashIgnoresMutableProfileButBindsTransaction(t *testing.T) {
	original := starTransactionHash(starRefundTransaction())
	renamed := starRefundTransaction()
	renamed.Receiver.User.FirstName, renamed.Receiver.User.LastName = "New", "Name"
	renamed.Receiver.User.Username, renamed.Receiver.User.LanguageCode = "renamed", "zh-hans"
	if starTransactionHash(renamed) != original {
		t.Fatal("mutable payer profile invalidated durable history boundary")
	}
	for name, mutate := range map[string]func(*telegram.StarTransaction){
		"charge":    func(tx *telegram.StarTransaction) { tx.ID = "different" },
		"amount":    func(tx *telegram.StarTransaction) { tx.Amount-- },
		"fraction":  func(tx *telegram.StarTransaction) { tx.NanostarAmount++ },
		"date":      func(tx *telegram.StarTransaction) { tx.Date++ },
		"direction": func(tx *telegram.StarTransaction) { tx.Source, tx.Receiver = tx.Receiver, nil },
		"partner":   func(tx *telegram.StarTransaction) { tx.Receiver.Type = "fragment" },
		"type":      func(tx *telegram.StarTransaction) { tx.Receiver.TransactionType = "paid_media_payment" },
		"owner":     func(tx *telegram.StarTransaction) { tx.Receiver.User.ID++ },
		"invoice":   func(tx *telegram.StarTransaction) { tx.Receiver.InvoicePayload = "different" },
	} {
		changed := starRefundTransaction()
		mutate(&changed)
		if starTransactionHash(changed) == original {
			t.Fatalf("%s changed without changing transaction proof hash", name)
		}
	}
}
