package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestUserDetailBuildsAggregateAndSelectsCurrentActiveCombo(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 18, 14, 0, 0, 0, time.UTC)
	remoteID := "remote-user"
	repository := &userWorkflowRepositoryStub{user: model.User{ID: "user-1", ProxyUserID: &remoteID},
		balance: model.TXBMoney(1234), accountErr: emby.ErrNotFound,
		purchases: []model.Purchase{
			{ID: "expired", Status: "expired", ValidFrom: now.Add(-60 * 24 * time.Hour), ValidUntil: now.Add(-30 * 24 * time.Hour)},
			{ID: "active", Status: "active", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(24 * time.Hour)},
			{ID: "queued", Status: "queued", ValidFrom: now.Add(24 * time.Hour), ValidUntil: now.Add(48 * time.Hour)},
		},
		payments: []model.PaymentOrder{{ID: "payment-1", UserID: "user-1"}},
		refunds:  []model.Refund{{ID: "refund-1"}}, operations: []model.OperationReceipt{{ID: "operation-1", Status: "queued"}}}
	service := NewUserWorkflows(repository, nil)
	service.now = func() time.Time { return now }

	detail, err := service.UserDetail(t.Context(), "user-1")
	if err != nil {
		t.Fatalf("UserDetail(): %v", err)
	}
	if detail.ActiveCombo == nil || detail.ActiveCombo.ID != "active" {
		t.Fatalf("active combo = %+v", detail.ActiveCombo)
	}
	if len(detail.Entitlements) != 3 || detail.Entitlements[0].ID != "queued" {
		t.Fatalf("sorted entitlements = %+v", detail.Entitlements)
	}
	if detail.EmbyAccounts == nil || len(detail.EmbyAccounts) != 0 {
		t.Fatalf("Emby accounts = %+v, want an empty array", detail.EmbyAccounts)
	}
	if detail.Synchronization.Status != "synchronized" || len(detail.Payments) != 1 ||
		len(detail.Refunds) != 1 || len(detail.Operations) != 1 {
		t.Fatalf("aggregate detail = %+v", detail)
	}
}

func TestSynchronizationForPrioritizesRecoveryFailure(t *testing.T) {
	t.Parallel()
	remoteID := "remote-user"
	state := synchronizationFor(model.User{ProxyUserID: &remoteID, RecoveryReason: "identity conflict"})
	if state.Status != "failed" || state.LastError == nil || *state.LastError != "identity conflict" {
		t.Fatalf("synchronization = %+v", state)
	}
}

type adminHourlyProjectionRepository struct {
	*adminCatalogRepository
	*hourlyPolicyTestRepository
	user        model.User
	balance     model.Money
	preference  model.HourlyExtraPreference
	credit      model.HourlyCreditDay
	creditErr   error
	outstanding []model.HourlyCreditReservation
	enrollments []database.AdminHourlyEnrollment
}

func (r *adminHourlyProjectionRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, nil
}

func (r *adminHourlyProjectionRepository) Balance(context.Context, string) (model.Money, error) {
	return r.balance, nil
}

func (r *adminHourlyProjectionRepository) HourlyExtraPreference(context.Context, string, int64) (model.HourlyExtraPreference, error) {
	return r.preference, nil
}

func (r *adminHourlyProjectionRepository) HourlyCreditDay(context.Context, string, int64, string) (model.HourlyCreditDay, error) {
	return r.credit, r.creditErr
}

func (r *adminHourlyProjectionRepository) OutstandingHourlyCredits(context.Context, string, int64, string, int) ([]model.HourlyCreditReservation, error) {
	return r.outstanding, nil
}

func (r *adminHourlyProjectionRepository) ListAdminHourlyEnrollments(context.Context, string, int64) ([]database.AdminHourlyEnrollment, error) {
	return r.enrollments, nil
}

func TestAdminHourlyProjectsPolicyWalletUsageAndSafeEnrollments(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	activated, paidThrough := now.Add(-2*time.Hour), now.Add(time.Hour)
	price := int64(25)
	repository := &adminHourlyProjectionRepository{
		adminCatalogRepository:     &adminCatalogRepository{},
		hourlyPolicyTestRepository: &hourlyPolicyTestRepository{adminSettingsRepository: newAdminSettingsRepository()},
		user:                       model.User{ID: "user-1", AccountGeneration: 3, AccountStatus: "active"},
		balance:                    model.TXBMoney(1_234),
		preference:                 model.HourlyExtraPreference{OwnerID: "user-1", AccountGeneration: 3, ExtraEnabled: true, AcceptedExtraPriceTXBMinor: &price},
		credit: model.HourlyCreditDay{FreeAllowanceBytes: 1_000, FreeAdmittedBytes: 400, FreeReservedBytes: 50,
			ExtraFundedBytes: 500, ExtraAdmittedBytes: 100, ExtraReservedBytes: 20},
		outstanding: []model.HourlyCreditReservation{{}},
		enrollments: []database.AdminHourlyEnrollment{{EnrollmentID: "enrollment-a", GroupID: "group-a", Name: "Group A",
			HourlyPriceTXBMinor: 125, OfferEnabled: true, Status: "active", ActivatedAt: &activated,
			PaidThrough: &paidThrough, CreatedAt: now.Add(-2 * time.Hour)}},
	}
	settings := NewSettingsService(repository, testVault(t))
	policy := model.DefaultHourlyPolicy()
	policy.SalesEnabled, policy.DailyAllowanceBytes, policy.ReducedRateBytesPerSecond, policy.ExtraGBPriceTXBMinor = true, 1_000, 1_024, 25
	saved, err := settings.SaveHourlyPolicy(context.Background(), "admin", "", policy)
	if err != nil {
		t.Fatal(err)
	}
	service := newAdminServiceForTest(repository, settings, nil, nil, nil)
	service.now = func() time.Time { return now }

	view, err := service.AdminHourly(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("AdminHourly(): %v", err)
	}
	if view.Policy != saved || view.Balance.MinorInt64() != 1_234 || !view.ExtraPreference.ExtraEnabled {
		t.Fatalf("projection header = %+v", view)
	}
	if view.Usage == nil || view.Usage.DayUTC != "2026-09-02" || view.Usage.FreeUsedBytes != 400 ||
		view.Usage.FreeReservedBytes != 50 || view.Usage.ExtraUsedBytes != 100 ||
		view.Usage.ExtraFundedBytes != 500 || !view.Usage.AccountingPending {
		t.Fatalf("usage = %+v", view.Usage)
	}
	if len(view.Squads) != 1 {
		t.Fatalf("squads = %+v", view.Squads)
	}
	squad := view.Squads[0]
	if squad.EnrollmentID != "enrollment-a" || squad.GroupID != "group-a" || squad.Name != "Group A" ||
		squad.HourlyPrice.MinorInt64() != 125 || !squad.OfferEnabled || squad.Status != "active" ||
		squad.PaidThrough == nil || squad.CooldownUntil != nil {
		t.Fatalf("squad = %+v", squad)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"proxyUserID", "periodId", "activationKey", "fingerprint", "accountGeneration"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestAdminHourlyFailsClosedWithoutProjectionStorage(t *testing.T) {
	repository := &adminCatalogRepository{}
	settings := NewSettingsService(newAdminSettingsRepository(), testVault(t))
	service := newAdminServiceForTest(repository, settings, nil, nil, nil)
	if _, err := service.AdminHourly(context.Background(), "user-1"); !errors.Is(err, ErrHourlyPolicyUnavailable) {
		t.Fatalf("AdminHourly() = %v", err)
	}
}
