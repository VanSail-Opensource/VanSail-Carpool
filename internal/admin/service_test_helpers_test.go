package admin

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"time"
)

type adminAuditCall struct {
	action     string
	targetType string
	targetID   string
	detail     string
}

type adminCatalogRepository struct {
	savedCombo        model.Combo
	comboInput        database.ComboInput
	saveComboErr      error
	deletedComboID    string
	deleteComboErr    error
	savedSquad        model.SquadProduct
	squadLookupErr    error
	squadInput        database.SquadProductInput
	savedSquadInputs  []database.SquadProductInput
	saveSquadErr      error
	listedSquads      []model.SquadProduct
	listSquadsErr     error
	adjustedEntry     model.LedgerEntry
	adjustDelta       int64
	adjustReference   string
	adjustActor       string
	adjustUser        string
	adjustReason      string
	adjustErr         error
	paymentOrder      model.PaymentOrder
	paymentOrderErr   error
	refundedOrder     model.PaymentOrder
	refundActor       *string
	refundErr         error
	courtesyCredit    model.CourtesyCredit
	courtesyActor     string
	courtesyOrderID   string
	courtesyReason    string
	courtesyErr       error
	cancelledPurchase model.Purchase
	cancelledID       string
	cancelActor       string
	cancelReason      string
	cancelErr         error
	audits            []adminAuditCall
	auditErr          error
}

func (r *adminCatalogRepository) ListCombos(context.Context, bool) ([]model.Combo, error) {
	return nil, nil
}
func (r *adminCatalogRepository) SquadProductByID(_ context.Context, id string) (model.SquadProduct, error) {
	if r.squadLookupErr != nil {
		return model.SquadProduct{}, r.squadLookupErr
	}
	if r.savedSquad.ID != "" {
		return r.savedSquad, nil
	}
	return model.SquadProduct{ID: id, GroupID: "uuid", UpstreamPresent: true}, nil
}
func (r *adminCatalogRepository) SquadProductByGroupID(_ context.Context, uuid string) (model.SquadProduct, error) {
	if r.squadLookupErr != nil {
		return model.SquadProduct{}, r.squadLookupErr
	}
	if r.savedSquad.ID != "" {
		return r.savedSquad, nil
	}
	return model.SquadProduct{ID: "squad-existing", GroupID: uuid, UpstreamPresent: true}, nil
}
func (r *adminCatalogRepository) ListSquadProducts(context.Context, bool) ([]model.SquadProduct, error) {
	return r.listedSquads, r.listSquadsErr
}
func (r *adminCatalogRepository) ListUsers(context.Context, int) ([]model.User, error) {
	return nil, nil
}
func (r *adminCatalogRepository) AdjustAdminBalance(_ context.Context, actorID, userID string, delta int64, reference, reason string, _ time.Time) (model.LedgerEntry, error) {
	r.adjustActor, r.adjustUser, r.adjustReason = actorID, userID, reason
	r.adjustDelta, r.adjustReference = delta, reference
	if r.adjustErr != nil {
		return model.LedgerEntry{}, r.adjustErr
	}
	return r.adjustedEntry, r.auditErr
}

func (r *adminCatalogRepository) DeductAdminBalance(_ context.Context, actorID, userID string, amount int64, reference, reason string, _ time.Time) (model.LedgerEntry, error) {
	r.adjustActor, r.adjustUser, r.adjustReason = actorID, userID, reason
	r.adjustDelta, r.adjustReference = -amount, reference
	if r.adjustErr != nil {
		return model.LedgerEntry{}, r.adjustErr
	}
	return r.adjustedEntry, r.auditErr
}
func (r *adminCatalogRepository) ListPaymentOrders(context.Context, string, int) ([]model.PaymentOrder, error) {
	return nil, nil
}
func (r *adminCatalogRepository) PaymentOrderByID(context.Context, string) (model.PaymentOrder, error) {
	return r.paymentOrder, r.paymentOrderErr
}
func (r *adminCatalogRepository) RefundPayment(_ context.Context, actor *string, _ string, _ string, _ time.Time) (model.PaymentOrder, error) {
	r.refundActor = actor
	return r.refundedOrder, r.refundErr
}
func (r *adminCatalogRepository) CourtesyCreditPayment(_ context.Context, actorID, orderID, reason string, _ time.Time) (model.CourtesyCredit, error) {
	r.courtesyActor, r.courtesyOrderID, r.courtesyReason = actorID, orderID, reason
	return r.courtesyCredit, r.courtesyErr
}
func (r *adminCatalogRepository) ListRefunds(context.Context, int) ([]model.Refund, error) {
	return nil, nil
}
func (r *adminCatalogRepository) CancelAdminPurchase(_ context.Context, actorID, id, reason string, _ time.Time) (model.Purchase, error) {
	r.cancelActor, r.cancelledID, r.cancelReason = actorID, id, reason
	if r.cancelErr != nil {
		return model.Purchase{}, r.cancelErr
	}
	return r.cancelledPurchase, r.auditErr
}
func (r *adminCatalogRepository) ListBackupRuns(context.Context, int) ([]model.BackupRun, error) {
	return nil, nil
}
func (r *adminCatalogRepository) ListOutboxJobs(context.Context, int) ([]model.OutboxJob, error) {
	return nil, nil
}
func (r *adminCatalogRepository) ListAuditEvents(context.Context, int) ([]model.AuditEvent, error) {
	return nil, nil
}
func (r *adminCatalogRepository) AppendAudit(_ context.Context, _ *string, action, targetType, targetID, detail string, _ time.Time) error {
	r.audits = append(r.audits, adminAuditCall{action: action, targetType: targetType, targetID: targetID, detail: detail})
	return r.auditErr
}

type adminSquadImporter struct {
	squads []UpstreamSquad
	err    error
}

func (i *adminSquadImporter) ListInternalSquads(context.Context) ([]UpstreamSquad, error) {
	return i.squads, i.err
}

type adminBackupRunner struct {
	run   model.BackupRun
	err   error
	actor string
}

func (b *adminBackupRunner) RunForActor(_ context.Context, actorID string) (model.BackupRun, error) {
	b.actor = actorID
	return b.run, b.err
}

type adminRefunder struct {
	calls int
	err   error
}

func (r *adminRefunder) RefundProvider(context.Context, model.PaymentOrder) error {
	r.calls++
	return r.err
}

func newAdminServiceForTest(repository CatalogRepository, settings *SettingsService, importer SquadImporter, backups BackupRunner, refunder PaymentRefunder) *Service {
	service := NewService(repository, settings, importer, backups, refunder)
	service.now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
	return service
}
