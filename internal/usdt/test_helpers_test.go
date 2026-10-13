package usdt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

type testRepository struct {
	Repository
	mu          sync.Mutex
	admissions  int
	limit       int
	finishes    int
	lastFailure time.Time
	quote       *Quote
	addresses   []string
	cursor      Cursor
	ready       []Transfer
	settled     int
}

func (r *testRepository) AdmitUSDTRPC(_ context.Context, _ QuotaGroup, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.limit > 0 && r.admissions >= r.limit {
		return &Pause{Code: "USDT_RPC_QUOTA_EXHAUSTED", Until: time.Now().Add(time.Hour)}
	}
	r.admissions++
	return nil
}
func (r *testRepository) FinishUSDTRPC(_ context.Context, _ string, until time.Time, ok bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finishes++
	if !ok {
		r.lastFailure = until
	}
	return nil
}
func (r *testRepository) USDTQuotaStatus(_ context.Context, groups []QuotaGroup, _ time.Time) ([]QuotaStatus, error) {
	q := []QuotaStatus{}
	for _, g := range groups {
		q = append(q, QuotaStatus{ID: g.ID})
	}
	return q, nil
}
func (r *testRepository) USDTQuote(context.Context, string) (Quote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.quote != nil {
		return *r.quote, nil
	}
	return Quote{}, ErrQuoteNotFound
}
func (r *testRepository) ReserveUSDTQuote(_ context.Context, id string, spec NetworkSpec, addresses []string, base string) (Quote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	atomic, err := Atomic(base, spec.Decimals)
	if err != nil {
		return Quote{}, err
	}
	q := Quote{OrderID: id, Network: spec.Network, Recipient: addresses[0], Amount: base, Token: spec.Token, AtomicAmount: atomic.String()}
	r.quote = &q
	return q, nil
}
func (r *testRepository) USDTWatchAddresses(context.Context, string) ([]string, error) {
	return r.addresses, nil
}
func (r *testRepository) USDTCursor(context.Context, string) (Cursor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cursor, nil
}
func (r *testRepository) SaveUSDTBatch(_ context.Context, _ string, b Batch) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cursor = b.Cursor
	return nil
}
func (r *testRepository) USDTReady(context.Context, int) ([]Transfer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Transfer(nil), r.ready...), nil
}
func (r *testRepository) SettleUSDTTransfer(_ context.Context, _ string, id string, _, _ string) (model.PaymentOrder, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settled++
	r.ready = nil
	return model.PaymentOrder{ID: id, Status: "paid"}, true, nil
}
func (r *testRepository) USDTReview(context.Context, string, int) (ReviewPage, error) {
	return ReviewPage{Items: []Transfer{}}, nil
}

type testSettings struct {
	mu  sync.Mutex
	raw string
}

func (s *testSettings) Optional(context.Context, string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.raw, nil
}
func (s *testSettings) Put(_ context.Context, _, _ string, raw string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.raw = raw
	return nil
}

func testClient(t *testing.T, network string, handler http.Handler) (*client, *testRepository) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r := &testRepository{}
	executor := newExecutor(r)
	if err := executor.configure(ctx, []QuotaGroup{{ID: "shared", RequestsPerSecond: 50, RequestsPerMinute: 3000, RequestsPerDay: 20000, Concurrency: 1}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = executor.close(context.Background()) })
	spec, _ := Network(network)
	c := newClient(ctx, executor, Endpoint{ID: "primary", Group: "shared", RPCURL: server.URL, IndexerURL: server.URL}, spec)
	t.Cleanup(c.close)
	return c, r
}
func sendJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(err)
	}
}
