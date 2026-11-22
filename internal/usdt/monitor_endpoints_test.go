package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestScannerKeepsHealthyFallbackAndRecoversWhenItFails(t *testing.T) {
	t.Parallel()
	var firstFailed, secondFailed atomic.Bool
	firstFailed.Store(true)
	first, firstCalls := failoverScanner(t, &firstFailed)
	second, secondCalls := failoverScanner(t, &secondFailed)
	readers := []*scanner{first, second}
	cursor := Cursor{Value: "100", Hash: "0x" + strings.Repeat("a", 64)}
	owners := []string{"0x" + strings.Repeat("1", 40)}
	for range 2 {
		batch, selected, err := scanEndpoints(context.Background(), readers, cursor, owners)
		if err != nil || selected != second || batch.Cursor != cursor || !batch.CaughtUp {
			t.Fatal("healthy fallback did not retain verified progress", batch, selected, err)
		}
	}
	if firstCalls.Load() != 1 || secondCalls.Load() != 2 {
		t.Fatal("failed priority endpoint was retried before healthy fallback", firstCalls.Load(), secondCalls.Load())
	}
	firstFailed.Store(false)
	secondFailed.Store(true)
	batch, selected, err := scanEndpoints(context.Background(), readers, cursor, owners)
	if err != nil || selected != first || batch.Cursor != cursor || readers[0] != first {
		t.Fatal("failed fallback did not recover the original endpoint", batch, selected, err)
	}
	firstFailed.Store(true)
	batch, selected, err = scanEndpoints(context.Background(), readers, cursor, owners)
	if !errors.Is(err, ErrRPC) || selected != nil || batch.Cursor.Value != "" {
		t.Fatal("total endpoint failure advanced progress", batch, selected, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := firstCalls.Load() + secondCalls.Load()
	if _, _, err := scanEndpoints(ctx, readers, cursor, owners); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled scan did not stop", err)
	}
	if calls != firstCalls.Load()+secondCalls.Load() {
		t.Fatal("cancelled scan contacted a provider")
	}
}

func TestUnchangedFinalizedEVMHeadChecksHashWithoutAnotherRPC(t *testing.T) {
	t.Parallel()
	var failed atomic.Bool
	reader, calls := failoverScanner(t, &failed)
	cursor := Cursor{Value: "100", Hash: "0x" + strings.Repeat("b", 64)}
	if _, err := reader.scan(context.Background(), cursor, nil); !errors.Is(err, ErrProof) {
		t.Fatal("changed finalized block hash was accepted", err)
	}
	if calls.Load() != 1 {
		t.Fatal("same finalized head was fetched twice", calls.Load())
	}
}

func failoverScanner(t *testing.T, failed *atomic.Bool) (*scanner, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	client, _ := testClient(t, "ethereum", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failed.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var request struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Method != "eth_getBlockByNumber" || len(request.Params) != 2 || request.Params[0] != "finalized" {
			t.Error("unexpected or non-finalized RPC", request.Method, request.Params)
		}
		sendJSON(w, map[string]any{"id": request.ID, "result": evmBlock{Number: "0x64", Hash: "0x" + strings.Repeat("a", 64)}})
	}))
	return &scanner{client: client, checked: true}, &calls
}
