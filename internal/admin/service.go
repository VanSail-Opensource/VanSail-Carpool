package admin

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"time"
)

// CatalogRepository is the domain-safe database surface available to admins.
type CatalogRepository interface {
	SaveCombo(context.Context, database.ComboInput) (model.Combo, error)
	DeleteCombo(context.Context, string) error
	ListCombos(context.Context, bool) ([]model.Combo, error)
	SaveSquadProduct(context.Context, database.SquadProductInput) (model.SquadProduct, error)
	SquadProductByID(context.Context, string) (model.SquadProduct, error)
	SquadProductByRemnaUUID(context.Context, string) (model.SquadProduct, error)
	ListSquadProducts(context.Context, bool) ([]model.SquadProduct, error)
	RefreshImportedSquads(context.Context, []database.ImportedSquad) error
	ListUsers(context.Context, int) ([]model.User, error)
	AdjustBalance(context.Context, string, int64, string, string, time.Time) (model.LedgerEntry, error)
	DeductBalance(context.Context, string, int64, string, string, time.Time) (model.LedgerEntry, error)
	ListPaymentOrders(context.Context, string, int) ([]model.PaymentOrder, error)
	PaymentOrderByID(context.Context, string) (model.PaymentOrder, error)
	RefundPayment(context.Context, *string, string, string, time.Time) (model.PaymentOrder, error)
	ListRefunds(context.Context, int) ([]model.Refund, error)
	ListAllPurchases(context.Context, int) ([]model.Purchase, error)
	CancelPurchase(context.Context, string, string, time.Time) (model.Purchase, error)
	ListBackupRuns(context.Context, int) ([]model.BackupRun, error)
	ListOutboxJobs(context.Context, int) ([]model.OutboxJob, error)
	RetryOutboxJob(context.Context, string, time.Time) error
	ListAuditEvents(context.Context, int) ([]model.AuditEvent, error)
	AppendAudit(context.Context, *string, string, string, string, string, time.Time) error
}

// PaymentRefunder performs provider-side refund operations that have a documented API.
type PaymentRefunder interface {
	RefundProvider(context.Context, model.PaymentOrder) error
}

// UpstreamSquad is imported from Remnawave without granting direct mutation access.
type UpstreamSquad struct {
	UUID string
	Name string
}

// SquadImporter lists Remnawave internal squads.
type SquadImporter interface {
	ListInternalSquads(context.Context) ([]UpstreamSquad, error)
}

// BackupRunner creates and verifies one online backup.
type BackupRunner interface {
	Run(context.Context) (model.BackupRun, error)
}

type backupDeleter interface {
	Delete(context.Context, string, string) error
}

// Service exposes audited administrative operations.
type Service struct {
	repository CatalogRepository
	settings   *SettingsService
	importer   SquadImporter
	backups    BackupRunner
	refunder   PaymentRefunder
	now        func() time.Time
}

// NewService constructs an admin service.
func NewService(repository CatalogRepository, settings *SettingsService, importer SquadImporter, backups BackupRunner, refunder PaymentRefunder) *Service {
	return &Service{repository: repository, settings: settings, importer: importer, backups: backups, refunder: refunder, now: time.Now}
}

// DeleteBackup delegates safe file containment and restore-state checks to the
// backup lifecycle service.
func (s *Service) DeleteBackup(ctx context.Context, actorID, backupID string) error {
	deleter, ok := s.backups.(backupDeleter)
	if !ok {
		return errors.New("backup deletion is unavailable")
	}
	return deleter.Delete(ctx, backupID, actorID)
}

// Settings returns masked write-only credentials.
func (s *Service) Settings(ctx context.Context) ([]model.Setting, error) {
	return s.settings.SafeList(ctx)
}

// PutSetting validates and audits one config mutation.
func (s *Service) PutSetting(ctx context.Context, actorID, key, value string) error {
	if err := s.settings.Put(ctx, actorID, key, value); err != nil {
		return err
	}
	return s.audit(ctx, actorID, "setting.update", "setting", key, map[string]any{"secret": settingDefinitions[key].Secret})
}
