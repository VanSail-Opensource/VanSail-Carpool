package admin

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"testing"
)

func TestAdminAdjustBalance(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		delta  int64
		reason string
	}{{name: "zero", reason: "reason"}, {name: "blank reason", delta: 100, reason: " "}} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := newAdminServiceForTest(&adminCatalogRepository{}, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{}).AdjustBalance(context.Background(), "admin", "user", test.delta, test.reason); err == nil {
				t.Fatal("AdjustBalance() unexpectedly succeeded")
			}
		})
	}

	repository := &adminCatalogRepository{adjustedEntry: model.LedgerEntry{ID: "ledger-1"}}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})
	entry, err := service.AdjustBalance(context.Background(), "admin", "user", -250, " correction ")
	if err != nil || entry.ID != "ledger-1" || repository.adjustReference == "" || repository.adjustDelta != -250 || repository.audits[0].action != "balance.adjust" {
		t.Fatalf("AdjustBalance() = (%+v, %v), repo %+v", entry, err, repository)
	}
	repository.adjustErr = errors.New("adjust failure")
	if _, err := service.AdjustBalance(context.Background(), "admin", "user", 1, "reason"); err == nil {
		t.Fatal("AdjustBalance() ignored repository failure")
	}
	repository.adjustErr = nil
	repository.auditErr = errors.New("audit failure")
	if _, err := service.AdjustBalance(context.Background(), "admin", "user", 1, "reason"); err == nil {
		t.Fatal("AdjustBalance() ignored audit failure")
	}
}

func TestAdminRefund(t *testing.T) {
	t.Parallel()

	if _, err := newAdminServiceForTest(&adminCatalogRepository{}, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{}).Refund(context.Background(), "admin", "order", " "); err == nil {
		t.Fatal("Refund() accepted blank reason")
	}

	repository := &adminCatalogRepository{paymentOrder: model.PaymentOrder{ID: "order-1", Status: "paid"}, refundedOrder: model.PaymentOrder{ID: "order-1", Status: "refunded"}}
	refunder := &adminRefunder{}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, &adminBackupRunner{}, refunder)
	order, err := service.Refund(context.Background(), "admin", "order-1", "provider reversal")
	if err != nil || order.Status != "refunded" || refunder.calls != 1 || repository.refundActor == nil || *repository.refundActor != "admin" || repository.audits[0].action != "payment.refund" {
		t.Fatalf("Refund() = (%+v, %v), provider calls %d, actor %v", order, err, refunder.calls, repository.refundActor)
	}

	repository.paymentOrder.Status = "refunded"
	if _, err := service.Refund(context.Background(), "admin", "order-1", "replay"); err != nil || refunder.calls != 1 {
		t.Fatalf("Refund(replay) = %v, provider calls %d", err, refunder.calls)
	}

	testError := errors.New("failure")
	tests := []struct {
		name       string
		repository *adminCatalogRepository
		refunder   *adminRefunder
	}{
		{name: "lookup", repository: &adminCatalogRepository{paymentOrderErr: testError}, refunder: &adminRefunder{}},
		{name: "provider", repository: &adminCatalogRepository{paymentOrder: model.PaymentOrder{Status: "paid"}}, refunder: &adminRefunder{err: testError}},
		{name: "database", repository: &adminCatalogRepository{paymentOrder: model.PaymentOrder{Status: "paid"}, refundErr: testError}, refunder: &adminRefunder{}},
		{name: "audit", repository: &adminCatalogRepository{paymentOrder: model.PaymentOrder{Status: "paid"}, auditErr: testError}, refunder: &adminRefunder{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := newAdminServiceForTest(test.repository, nil, &adminSquadImporter{}, &adminBackupRunner{}, test.refunder).Refund(context.Background(), "admin", "order", "reason"); err == nil {
				t.Fatal("Refund() unexpectedly succeeded")
			}
		})
	}
}

func TestAdminCourtesyCredit(t *testing.T) {
	t.Parallel()

	repository := &adminCatalogRepository{courtesyCredit: model.CourtesyCredit{ID: "credit-1", PaymentOrderID: "order-1"}}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})
	for _, reason := range []string{" ", "ok"} {
		if _, err := service.CourtesyCredit(context.Background(), "admin", "order-1", reason); err == nil {
			t.Fatalf("CourtesyCredit(%q) unexpectedly succeeded", reason)
		}
	}
	credit, err := service.CourtesyCredit(context.Background(), "admin", "order-1", "provider failure confirmed")
	if err != nil || credit.ID != "credit-1" || repository.courtesyActor != "admin" || repository.courtesyOrderID != "order-1" || repository.courtesyReason != "provider failure confirmed" {
		t.Fatalf("CourtesyCredit() = (%+v, %v), repository = %+v", credit, err, repository)
	}
	repository.courtesyErr = errors.New("credit failure")
	if _, err := service.CourtesyCredit(context.Background(), "admin", "order-1", "provider failure confirmed"); err == nil {
		t.Fatal("CourtesyCredit() ignored repository failure")
	}
}

func TestAdminCancelBackupAndRetry(t *testing.T) {
	t.Parallel()

	repository := &adminCatalogRepository{cancelledPurchase: model.Purchase{ID: "purchase-1", Status: "cancelled"}}
	backup := &adminBackupRunner{run: model.BackupRun{ID: "backup-1", Status: "complete"}}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, backup, &adminRefunder{})

	if _, err := service.CancelEntitlement(context.Background(), "admin", "purchase", " "); err == nil {
		t.Fatal("CancelEntitlement() accepted blank reason")
	}
	purchase, err := service.CancelEntitlement(context.Background(), "admin", "purchase-1", "requested")
	if err != nil || purchase.Status != "cancelled" || repository.cancelledID != "purchase-1" {
		t.Fatalf("CancelEntitlement() = (%+v, %v), ID %q", purchase, err, repository.cancelledID)
	}
	run, err := service.RunBackup(context.Background(), "admin")
	if err != nil || run.ID != "backup-1" {
		t.Fatalf("RunBackup() = (%+v, %v)", run, err)
	}
	if err := service.RetryJob(context.Background(), "admin", "job-1"); err != nil || repository.retriedJobID != "job-1" {
		t.Fatalf("RetryJob() = %v, ID %q", err, repository.retriedJobID)
	}

	testError := errors.New("failure")
	repository.cancelErr = testError
	if _, err := service.CancelEntitlement(context.Background(), "admin", "purchase", "reason"); err == nil {
		t.Fatal("CancelEntitlement() ignored repository failure")
	}
	repository.cancelErr = nil
	backup.err = testError
	if got, err := service.RunBackup(context.Background(), "admin"); err == nil || got.ID != "backup-1" {
		t.Fatalf("RunBackup(error) = (%+v, %v)", got, err)
	}
	backup.err = nil
	repository.retryErr = testError
	if err := service.RetryJob(context.Background(), "admin", "job"); err == nil {
		t.Fatal("RetryJob() ignored repository failure")
	}
	repository.retryErr = nil
	repository.auditErr = testError
	if _, err := service.CancelEntitlement(context.Background(), "admin", "purchase", "reason"); err == nil {
		t.Fatal("CancelEntitlement() ignored audit failure")
	}
	if _, err := service.RunBackup(context.Background(), "admin"); err == nil {
		t.Fatal("RunBackup() ignored audit failure")
	}
	if err := service.RetryJob(context.Background(), "admin", "job"); err == nil {
		t.Fatal("RetryJob() ignored audit failure")
	}
}
