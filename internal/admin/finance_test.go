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
			if _, err := newAdminServiceForTest(&adminCatalogRepository{}, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{}).AdjustBalanceWithReference(context.Background(), "admin", "user", test.delta, "adjustment-key", test.reason); err == nil {
				t.Fatal("AdjustBalanceWithReference() unexpectedly succeeded")
			}
		})
	}

	repository := &adminCatalogRepository{adjustedEntry: model.LedgerEntry{ID: "ledger-1"}}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})
	entry, err := service.AdjustBalanceWithReference(context.Background(), "admin", "user", -250, "adjustment-key", " correction ")
	if err != nil || entry.ID != "ledger-1" || repository.adjustReference != "adjustment-key" || repository.adjustDelta != -250 || repository.adjustActor != "admin" || repository.adjustUser != "user" || repository.adjustReason != "correction" {
		t.Fatalf("AdjustBalanceWithReference() = (%+v, %v), repo %+v", entry, err, repository)
	}
	repository.adjustErr = errors.New("adjust failure")
	if _, err := service.AdjustBalanceWithReference(context.Background(), "admin", "user", 1, "error-key", "reason"); err == nil {
		t.Fatal("AdjustBalanceWithReference() ignored repository failure")
	}
	repository.adjustErr = nil
	repository.auditErr = errors.New("audit failure")
	if _, err := service.AdjustBalanceWithReference(context.Background(), "admin", "user", 1, "error-key", "reason"); err == nil {
		t.Fatal("AdjustBalanceWithReference() ignored audit failure")
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

func TestAdminCancelAndBackup(t *testing.T) {
	t.Parallel()

	repository := &adminCatalogRepository{cancelledPurchase: model.Purchase{ID: "purchase-1", Status: "cancelled"}}
	backup := &adminBackupRunner{run: model.BackupRun{ID: "backup-1", Status: "complete"}}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, backup, &adminRefunder{})

	if _, err := service.CancelEntitlement(context.Background(), "admin", "purchase", " "); err == nil {
		t.Fatal("CancelEntitlement() accepted blank reason")
	}
	purchase, err := service.CancelEntitlement(context.Background(), "admin", "purchase-1", "requested")
	if err != nil || purchase.Status != "cancelled" || repository.cancelledID != "purchase-1" || repository.cancelActor != "admin" || repository.cancelReason != "requested" {
		t.Fatalf("CancelEntitlement() = (%+v, %v), ID %q", purchase, err, repository.cancelledID)
	}
	run, err := service.RunBackup(context.Background(), "admin")
	if err != nil || run.ID != "backup-1" {
		t.Fatalf("RunBackup() = (%+v, %v)", run, err)
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
	repository.auditErr = testError
	if _, err := service.CancelEntitlement(context.Background(), "admin", "purchase", "reason"); err == nil {
		t.Fatal("CancelEntitlement() ignored audit failure")
	}
	if _, err := service.RunBackup(context.Background(), "admin"); err == nil {
		t.Fatal("RunBackup() ignored audit failure")
	}
}
