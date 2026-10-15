package iplookup

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type soakRepository struct {
	usdt.Repository
	mu       sync.Mutex
	cursors  map[string]usdt.Cursor
	batches  atomic.Int64
	verified map[string]bool
}

func (r *soakRepository) USDTWatchAddresses(context.Context, string) ([]string, error) {
	return []string{soakRecipient}, nil
}
func (r *soakRepository) USDTCursor(_ context.Context, network string) (usdt.Cursor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.cursors[network]; ok {
		return c, nil
	}
	return usdt.Cursor{Value: "1", Hash: soakHash(1)}, nil
}
func (r *soakRepository) SaveUSDTBatch(_ context.Context, network string, b usdt.Batch) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cursors[network] = b.Cursor
	if len(b.Transfers) > 0 {
		r.batches.Add(1)
		r.verified[network] = true
	}
	return nil
}

func (r *soakRepository) verifiedNetworks() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.verified)
}
func (r *soakRepository) USDTReady(context.Context, int) ([]usdt.Transfer, error) { return nil, nil }

type soakSettings struct{ raw string }

func (s soakSettings) Optional(context.Context, string) (string, error)  { return s.raw, nil }
func (s soakSettings) Put(context.Context, string, string, string) error { return nil }
func startSoakScanners(t *testing.T, server *httptest.Server) (*soakRepository, func()) {
	t.Helper()
	cfg := usdt.DefaultConfig()
	cfg.Enabled = true
	cfg.Rate = "1"
	cfg.Concurrency = 8
	cfg.Networks = nil
	for _, spec := range usdt.Registry {
		if spec.Family != "evm" {
			continue
		}
		cfg.Networks = append(cfg.Networks, usdt.NetworkConfig{Network: spec.Network, Enabled: true, Addresses: []string{soakRecipient}, Endpoints: []usdt.Endpoint{
			{ID: "fail-" + spec.Network, RPCURL: server.URL + "/fail/" + spec.Network},
			{ID: spec.Network, RPCURL: server.URL + "/" + spec.Network, UseWS: true, WSURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/" + spec.Network},
		}})
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	repo := &soakRepository{cursors: map[string]usdt.Cursor{}, verified: map[string]bool{}}
	service := usdt.NewService(repo, soakSettings{raw: string(raw)})
	ctx, cancel := context.WithCancel(context.Background())
	service.Start(ctx)
	return repo, func() { cancel(); service.Wait() }
}
