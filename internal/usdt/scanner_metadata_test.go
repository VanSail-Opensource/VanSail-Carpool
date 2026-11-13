package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAptosInitializationRequiresAValidProcessorVersion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, row, version string
	}{
		{"missing", `{}`, ""},
		{"empty", `{"last_success_version":""}`, ""},
		{"overflow", `{"last_success_version":"9223372036854775808"}`, ""},
		{"zero", `{"last_success_version":"0"}`, "0"},
		{"current", `{"last_success_version":"100"}`, "100"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := testClient(t, "aptos", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"data":{"processor_status":[` + tc.row + `]}}`))
			}))
			batch, err := (&scanner{client: client, checked: true}).scan(context.Background(), Cursor{}, nil)
			if tc.version == "" {
				if err == nil || batch.Cursor.Value != "" || batch.CaughtUp {
					t.Fatal("incomplete processor status advertised readiness", batch, err)
				}
				return
			}
			var cursor aptosCursor
			if err != nil || !batch.CaughtUp || json.Unmarshal([]byte(batch.Cursor.Value), &cursor) != nil || cursor.Version != tc.version {
				t.Fatal("valid processor baseline rejected", batch, err)
			}
		})
	}
}

func TestSolanaMissingSignatureMetadataCannotAdvanceHistory(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		ready  bool
	}{
		{"execution status", func(v map[string]any) { delete(v, "err") }, false},
		{"signature", func(v map[string]any) { delete(v, "signature") }, false},
		{"slot", func(v map[string]any) { delete(v, "slot") }, false},
		{"old unknown status", func(v map[string]any) { v["slot"] = 99; delete(v, "err") }, false},
		{"boolean status", func(v map[string]any) { v["err"] = false }, false},
		{"numeric status", func(v map[string]any) { v["err"] = 0 }, false},
		{"empty variant", func(v map[string]any) { v["err"] = "" }, false},
		{"empty object", func(v map[string]any) { v["err"] = map[string]any{} }, false},
		{"array status", func(v map[string]any) { v["err"] = []any{} }, false},
		{"unit failure", func(v map[string]any) { v["err"] = "AccountNotFound" }, true},
		{"tagged failure", func(v map[string]any) { v["err"] = map[string]any{"InstructionError": []any{0, "InvalidArgument"}} }, true},
		{"known old success", func(v map[string]any) { v["slot"] = 99 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := map[string]any{"signature": "receiving-signature", "slot": 101, "err": nil, "confirmationStatus": "finalized"}
			tc.mutate(entry)
			client, _ := testClient(t, "solana", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     uint64 `json:"id"`
					Method string `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Method != "getSignaturesForAddress" {
					t.Error("invalid history triggered another proof request", request.Method, err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				sendJSON(w, map[string]any{"id": request.ID, "result": []any{entry}})
			}))
			owner := client.spec.Token
			account, err := associatedTokenAddress(owner, client.spec.Token)
			if err != nil {
				t.Fatal(err)
			}
			cursor := Cursor{Value: cursorJSON(solCursor{Baseline: 100, RefreshAt: time.Now().Unix(),
				Accounts: map[string]solHistory{account: {Owner: owner}}})}
			batch, err := (&scanner{client: client, checked: true}).scan(context.Background(), cursor, []string{owner})
			if tc.ready {
				if err != nil || batch.Cursor.Value == "" || !batch.CaughtUp || len(batch.Transfers) != 0 {
					t.Fatal("known failure or historical boundary blocked recovery", batch, err)
				}
			} else if !errors.Is(err, ErrProof) || batch.Cursor.Value != "" || len(batch.Transfers) != 0 || batch.CaughtUp {
				t.Fatal("unknown signature evidence advanced history", batch, err)
			}
		})
	}
}

func TestTronMissingExecutionReceiptRemainsUnverified(t *testing.T) {
	t.Parallel()
	spec, _ := Network("tron")
	hash := strings.Repeat("a", 64)
	receipt := tronReceipt{ID: hash, BlockNumber: 100, Timestamp: time.Now().Add(-time.Minute).UnixMilli()}
	for _, result := range []string{"", "DEFAULT", "not-an-execution-result"} {
		receipt.Receipt.Result = result
		if transfers, err := decodeTronReceipt(spec, receipt, hash, nil); !errors.Is(err, ErrProof) || len(transfers) != 0 {
			t.Fatal("unknown execution result was treated as a known failure", result, transfers, err)
		}
	}
	for _, result := range []string{"REVERT", "UNKNOWN"} {
		receipt.Receipt.Result = result
		if transfers, err := decodeTronReceipt(spec, receipt, hash, nil); err != nil || len(transfers) != 0 {
			t.Fatal("explicit failed transaction stopped history recovery", result, transfers, err)
		}
	}
}
