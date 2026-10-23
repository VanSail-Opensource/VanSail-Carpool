package admin

import (
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
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
