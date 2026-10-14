package billing

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"net/url"
	"time"
)

func ptrString(value string) *string { return &value }

type billingRepository struct {
	orders         map[string]model.PaymentOrder
	user           model.User
	createErr      error
	updateErr      error
	lookupErr      error
	userErr        error
	failed         bool
	updatedOrderID string
	settleDedupe   string
	settleHash     string
	paymentForUser model.PaymentOrder
	pollUserID     string
}

func newBillingRepository() *billingRepository {
	return &billingRepository{orders: make(map[string]model.PaymentOrder), user: model.User{ID: "user-1"}}
}

func (r *billingRepository) CreatePaymentOrder(_ context.Context, order model.PaymentOrder) (model.PaymentOrder, error) {
	if r.createErr != nil {
		return model.PaymentOrder{}, r.createErr
	}
	order.ID = "order-1"
	r.orders[order.ID] = order
	return order, nil
}

func (r *billingRepository) UpdatePaymentCheckout(_ context.Context, orderID string, tradeID, paymentURL, qrPayload *string, amount, currency string, expires time.Time) (model.PaymentOrder, error) {
	if r.updateErr != nil {
		return model.PaymentOrder{}, r.updateErr
	}
	r.updatedOrderID = orderID
	order := r.orders[orderID]
	order.Status = "pending"
	order.ProviderTradeID = tradeID
	order.PaymentURL = paymentURL
	order.QRPayload = qrPayload
	order.PayableAmount = amount
	order.PayableCurrency = currency
	order.ExpiresAt = expires
	r.orders[orderID] = order
	return order, nil
}

func (r *billingRepository) FailPaymentOrder(context.Context, string) error {
	r.failed = true
	return nil
}

func (r *billingRepository) PaymentOrderByID(_ context.Context, id string) (model.PaymentOrder, error) {
	if r.lookupErr != nil {
		return model.PaymentOrder{}, r.lookupErr
	}
	return r.orders[id], nil
}

func (r *billingRepository) PaymentOrderForUser(_ context.Context, _, userID string) (model.PaymentOrder, error) {
	r.pollUserID = userID
	return r.paymentForUser, nil
}

func (r *billingRepository) SettlePayment(_ context.Context, _, dedupeKey, payloadHash, orderID, _, _ string, _ time.Time) (model.PaymentOrder, bool, error) {
	r.settleDedupe = dedupeKey
	r.settleHash = payloadHash
	return r.orders[orderID], true, nil
}

func (r *billingRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, r.userErr
}

type billingSettings struct {
	values map[string]string
	errs   map[string]error
}

func (s *billingSettings) Plaintext(_ context.Context, key string) (string, error) {
	if s.errs != nil && s.errs[key] != nil {
		return "", s.errs[key]
	}
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	switch key {
	case "billing.ezpay.methods":
		return "alipay", nil
	case "billing.bepusdt.methods":
		return "usdt.trc20", nil
	case "billing.bepusdt.api_token":
		return "test-callback-secret", nil
	default:
		return "", nil
	}
}

func (s *billingSettings) Optional(ctx context.Context, key string) (string, error) {
	return s.Plaintext(ctx, key)
}

type billingGateway struct {
	request  ProviderCreateRequest
	checkout ProviderCheckout
	err      error
}

type bepusdtRetryGateway struct {
	calls    int
	orderIDs []string
	firstErr error
}

func (g *bepusdtRetryGateway) Create(_ context.Context, request ProviderCreateRequest) (ProviderCheckout, error) {
	g.calls++
	g.orderIDs = append(g.orderIDs, request.OrderID)
	if g.calls == 1 {
		return ProviderCheckout{}, g.firstErr
	}
	return ProviderCheckout{}, nil
}

func (g *billingGateway) Create(_ context.Context, request ProviderCreateRequest) (ProviderCheckout, error) {
	g.request = request
	return g.checkout, g.err
}

func newBillingServiceForTest(repository Repository, settings Settings, gateway Gateway) *Service {
	publicURL, _ := url.Parse("https://example.test/base")
	service := NewService(repository, settings, gateway, publicURL)
	service.now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
	return service
}
