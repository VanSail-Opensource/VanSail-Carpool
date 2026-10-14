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
	markedMissing     bool
	markMissingErr    error
	importedSquads    []database.ImportedSquad
	refreshErr        error
	listedSquads      []model.SquadProduct
	listSquadsErr     error
	adjustedEntry     model.LedgerEntry
	adjustDelta       int64
	adjustReference   string
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
	cancelErr         error
	retriedJobID      string
	retryErr          error
	audits            []adminAuditCall
	auditErr          error
}

func (r *adminCatalogRepository) SaveCombo(_ context.Context, input database.ComboInput) (model.Combo, error) {
	r.comboInput = input
	return r.savedCombo, r.saveComboErr
}
func (r *adminCatalogRepository) DeleteCombo(_ context.Context, id string) error {
	r.deletedComboID = id
	return r.deleteComboErr
}
func (r *adminCatalogRepository) ListCombos(context.Context, bool) ([]model.Combo, error) {
	return nil, nil
}
func (r *adminCatalogRepository) SaveSquadProduct(_ context.Context, input database.SquadProductInput) (model.SquadProduct, error) {
	r.squadInput = input
	r.savedSquadInputs = append(r.savedSquadInputs, input)
	if r.saveSquadErr != nil {
		return model.SquadProduct{}, r.saveSquadErr
	}
	if r.savedSquad.ID != "" {
		return r.savedSquad, nil
	}
	return model.SquadProduct{ID: input.RemnaSquadUUID, Name: input.Name}, nil
}
func (r *adminCatalogRepository) SquadProductByID(_ context.Context, id string) (model.SquadProduct, error) {
	if r.squadLookupErr != nil {
		return model.SquadProduct{}, r.squadLookupErr
	}
	if r.savedSquad.ID != "" {
		return r.savedSquad, nil
	}
	return model.SquadProduct{ID: id, RemnaSquadUUID: "uuid", UpstreamPresent: true}, nil
}
func (r *adminCatalogRepository) SquadProductByRemnaUUID(_ context.Context, uuid string) (model.SquadProduct, error) {
	if r.squadLookupErr != nil {
		return model.SquadProduct{}, r.squadLookupErr
	}
	if r.savedSquad.ID != "" {
		return r.savedSquad, nil
	}
	return model.SquadProduct{ID: "squad-existing", RemnaSquadUUID: uuid, UpstreamPresent: true}, nil
}
func (r *adminCatalogRepository) ListSquadProducts(context.Context, bool) ([]model.SquadProduct, error) {
	return r.listedSquads, r.listSquadsErr
}
func (r *adminCatalogRepository) MarkAllSquadsMissing(context.Context) error {
	r.markedMissing = true
	return r.markMissingErr
}
func (r *adminCatalogRepository) RefreshImportedSquads(_ context.Context, squads []database.ImportedSquad) error {
	r.importedSquads = append([]database.ImportedSquad(nil), squads...)
	return r.refreshErr
}
func (r *adminCatalogRepository) ListUsers(context.Context, int) ([]model.User, error) {
	return nil, nil
}
func (r *adminCatalogRepository) AdjustBalance(_ context.Context, _ string, delta int64, reference, _ string, _ time.Time) (model.LedgerEntry, error) {
	r.adjustDelta, r.adjustReference = delta, reference
	return r.adjustedEntry, r.adjustErr
}

func (r *adminCatalogRepository) DeductBalance(_ context.Context, _ string, amount int64, reference, _ string, _ time.Time) (model.LedgerEntry, error) {
	r.adjustDelta, r.adjustReference = -amount, reference
	return r.adjustedEntry, r.adjustErr
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
func (r *adminCatalogRepository) ListAllPurchases(context.Context, int) ([]model.Purchase, error) {
	return nil, nil
}
func (r *adminCatalogRepository) CancelPurchase(_ context.Context, id, _ string, _ time.Time) (model.Purchase, error) {
	r.cancelledID = id
	return r.cancelledPurchase, r.cancelErr
}
func (r *adminCatalogRepository) ListBackupRuns(context.Context, int) ([]model.BackupRun, error) {
	return nil, nil
}
func (r *adminCatalogRepository) ListOutboxJobs(context.Context, int) ([]model.OutboxJob, error) {
	return nil, nil
}
func (r *adminCatalogRepository) RetryOutboxJob(_ context.Context, id string, _ time.Time) error {
	r.retriedJobID = id
	return r.retryErr
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
	run model.BackupRun
	err error
}

func (b *adminBackupRunner) Run(context.Context) (model.BackupRun, error) { return b.run, b.err }

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
