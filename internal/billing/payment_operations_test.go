package billing

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type ambiguousPaymentGateway struct {
	calls int
}

func (g *ambiguousPaymentGateway) Create(context.Context, ProviderCreateRequest) (ProviderCheckout, error) {
	g.calls++
	return ProviderCheckout{}, errors.New("provider outcome is unknown")
}

func TestPaymentOperationReplayConflictAndAtomicQueue(t *testing.T) {
	ctx := context.Background()
	store, user := newPaymentOperationStore(t, 80_001)
	service := paymentOperationService(store, &billingGateway{})

	created, err := service.QueueOrder(ctx, user, "ezpay:alipay", 250, "create-key")
	if err != nil {
		t.Fatalf("QueueOrder(): %v", err)
	}
	replayed, err := service.QueueOrder(ctx, user, "ezpay:alipay", 250, "create-key")
	if err != nil || replayed.Operation.ID != created.Operation.ID || replayed.PaymentOrderID != created.PaymentOrderID {
		t.Fatalf("QueueOrder() replay = (%+v, %v), want operation %q", replayed, err, created.Operation.ID)
	}
	if _, err := service.QueueOrder(ctx, user, "ezpay:alipay", 251, "create-key"); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("QueueOrder() conflict error = %v, want %v", err, database.ErrConflict)
	}

	assertPaymentOperationCount(t, store, "payment_orders", 1)
	assertPaymentOperationCount(t, store, "outbox_jobs", 1)
}

func TestPaymentOperationAmbiguityIsNotRetriedAndCallbackResolves(t *testing.T) {
	ctx := context.Background()
	store, user := newPaymentOperationStore(t, 80_002)
	gateway := &ambiguousPaymentGateway{}
	service := paymentOperationService(store, gateway)
	queued, err := service.QueueOrder(ctx, user, "ezpay:alipay", 250, "ambiguous-key")
	if err != nil {
		t.Fatalf("QueueOrder(): %v", err)
	}
	worker, err := NewOperationWorker(service)
	if err != nil {
		t.Fatalf("NewOperationWorker(): %v", err)
	}
	operation, err := store.ProviderOperationByID(ctx, queued.Operation.ID)
	if err != nil {
		t.Fatalf("ProviderOperationByID(): %v", err)
	}
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{}); err != nil {
		t.Fatalf("HandleProviderOperation(): %v", err)
	}
	operation, err = store.ProviderOperationByID(ctx, queued.Operation.ID)
	if err != nil || operation.Receipt.Status != string(providerops.StatusPendingReview) || gateway.calls != 1 {
		t.Fatalf("ambiguous operation = (%+v, %v), provider calls = %d", operation, err, gateway.calls)
	}
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{}); err != nil || gateway.calls != 1 {
		t.Fatalf("terminal replay = %v, provider calls = %d", err, gateway.calls)
	}
	if err := store.ResolvePaymentCreateOperation(ctx, queued.PaymentOrderID, "trade-1", time.Now().UTC()); err != nil {
		t.Fatalf("ResolvePaymentCreateOperation(): %v", err)
	}
	receipt, err := store.ProviderOperationForOwner(ctx, queued.Operation.ID, user.ID)
	if err != nil || receipt.Status != string(providerops.StatusSucceeded) {
		t.Fatalf("resolved receipt = (%+v, %v)", receipt, err)
	}
}

func newPaymentOperationStore(t *testing.T, telegramID int64) (*database.Store, model.User) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "payment-operations.db"))
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := database.NewStore(db)
	user, _, err := store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: telegramID, FirstName: "Payment"}, false)
	if err != nil {
		t.Fatalf("UpsertTelegramUser(): %v", err)
	}
	return store, user
}

func paymentOperationService(store *database.Store, gateway Gateway) *Service {
	return newBillingServiceForTest(store, &billingSettings{values: map[string]string{
		"billing.ezpay.enabled": "true", "billing.rate.txb_per_cny": "1",
	}}, gateway)
}

func assertPaymentOperationCount(t *testing.T, store *database.Store, table string, want int) {
	t.Helper()
	var got int
	if err := store.DB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}
