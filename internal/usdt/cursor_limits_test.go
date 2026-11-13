package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestScannerRejectsOversizedProgressBeforeProviderWork(t *testing.T) {
	t.Parallel()
	reader := &scanner{} // No client may be accessed before the size gate.
	for _, cursor := range []Cursor{
		{Value: strings.Repeat("x", MaxCursorBytes+1)},
		{Hash: strings.Repeat("x", MaxCursorHashBytes+1)},
	} {
		batch, err := reader.scan(context.Background(), cursor, nil)
		if !errors.Is(err, ErrCapacity) || batch.Cursor.Value != "" || batch.CaughtUp {
			t.Fatal("oversized progress reached provider work or advanced the cursor", batch, err)
		}
	}
	if err := ValidateCursor(Cursor{Value: strings.Repeat("x", MaxCursorBytes), Hash: strings.Repeat("x", MaxCursorHashBytes)}); err != nil {
		t.Fatal("exact storage bounds were rejected", err)
	}
}

func TestCursorGraphLimitRejectsBeforeTypedAllocation(t *testing.T) {
	t.Parallel()
	raw := `{"accounts":{` + strings.Repeat(`"duplicate":{},`, 40000) + `"last":{}}}`
	var cursor solCursor
	if err := decodeCursor(raw, &cursor); !errors.Is(err, ErrCapacity) || cursor.Accounts != nil {
		t.Fatal("unbounded graph reached typed cursor decoding", err, len(cursor.Accounts))
	}
}

func TestSolanaDiscoveryCapacityPreservesArchivedPaymentHistory(t *testing.T) {
	t.Parallel()
	spec, _ := Network("solana")
	owner := spec.Token
	accounts := make(map[string]solHistory, maxSolanaCursorAccounts)
	for i := range maxSolanaCursorAccounts {
		accounts[fmt.Sprintf("archived-%04d", i)] = solHistory{Owner: owner, Last: "retained-payment-boundary"}
	}
	original := Cursor{Value: cursorJSON(solCursor{Baseline: 100, Accounts: accounts})}
	var providerCalls atomic.Int32
	client, _ := testClient(t, "solana", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Method != "getTokenAccountsByOwner" {
			t.Error("capacity overflow advanced into account history", request.Method, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		providerCalls.Add(1)
		sendJSON(w, map[string]any{"id": request.ID, "result": map[string]any{"value": []any{}}})
	}))
	batch, err := (&scanner{client: client, checked: true}).scan(context.Background(), original, []string{owner})
	if !errors.Is(err, ErrCapacity) || batch.Cursor.Value != "" || len(batch.Transfers) != 0 || providerCalls.Load() != 1 {
		t.Fatal("discovery evicted old histories or published an incomplete replacement", batch, err, providerCalls.Load())
	}
	var retained solCursor
	if err := decodeCursor(original.Value, &retained); err != nil || len(retained.Accounts) != maxSolanaCursorAccounts || retained.Accounts["archived-0000"].Last != "retained-payment-boundary" {
		t.Fatal("earlier durable progress was damaged", err)
	}
}

func TestTimeCursorInvalidIndexesAndWindowsCannotReachProvider(t *testing.T) {
	t.Parallel()
	for _, page := range []timePage{
		{Address: -1}, {Offset: -1}, {Start: -1}, {End: -1}, {Completed: -1},
		{Start: 2, End: 1}, {End: time.Now().Add(time.Hour).Unix()},
	} {
		if _, err := loadTimePage(Cursor{Value: cursorJSON(page)}); !errors.Is(err, ErrProof) {
			t.Fatal("invalid pagination state was accepted", page, err)
		}
	}
}
