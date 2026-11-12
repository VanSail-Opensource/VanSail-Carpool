package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestSolanaV1HistoryRetainsProofAndCursorRecovery(t *testing.T) {
	for _, tc := range []struct {
		name      string
		version   int
		failed    bool
		wantError error
	}{
		{name: "v0 remains supported", version: 0},
		{name: "v1 transfer advances cursor", version: 1},
		{name: "v1 failed transaction preserves cursor", version: 1, failed: true, wantError: ErrProof},
		{name: "unknown version preserves cursor", version: 2, wantError: ErrRPC},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, _ := Network("usdc.solana")
			owner, signature := spec.Token, "versioned-receiving-signature"
			account, err := associatedTokenAddress(owner, spec.Token)
			if err != nil {
				t.Fatal(err)
			}
			proof := solanaProofFixture(spec, owner, account, signature)
			if tc.failed {
				proof.Meta.Err = json.RawMessage(`{"InstructionError":[0,"failed"]}`)
			}
			raw, err := json.Marshal(proof)
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			if err := json.Unmarshal(raw, &response); err != nil {
				t.Fatal(err)
			}
			response["version"] = tc.version
			if tc.version == 1 {
				message := response["transaction"].(map[string]any)["message"].(map[string]any)
				message["transactionConfig"] = map[string]any{"computeUnitLimit": 200000}
			}
			client, _ := testClient(t, spec.Rail, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     uint64            `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				switch request.Method {
				case "getSignaturesForAddress":
					sendJSON(w, map[string]any{"id": request.ID, "result": []any{map[string]any{
						"signature": signature, "slot": 101, "err": nil, "confirmationStatus": "finalized",
					}}})
				case "getTransaction":
					var options struct {
						Encoding   string `json:"encoding"`
						Commitment string `json:"commitment"`
						Version    int    `json:"maxSupportedTransactionVersion"`
					}
					if len(request.Params) != 2 || json.Unmarshal(request.Params[1], &options) != nil || options.Encoding != "jsonParsed" || options.Commitment != "finalized" {
						t.Error("transaction fetch lost its parsed finalized proof contract")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if options.Version < tc.version {
						sendJSON(w, map[string]any{"id": request.ID, "error": map[string]any{"code": -32015}})
						return
					}
					sendJSON(w, map[string]any{"id": request.ID, "result": response})
				default:
					t.Errorf("unexpected method %s", request.Method)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			cursor := Cursor{Value: cursorJSON(solCursor{Baseline: 100, RefreshAt: time.Now().Unix(), Accounts: map[string]solHistory{
				account: {Owner: owner},
			}})}
			batch, err := (&scanner{client: client, checked: true}).scan(context.Background(), cursor, []string{owner})
			if tc.wantError != nil {
				if !errors.Is(err, tc.wantError) || batch.Cursor.Value != "" || len(batch.Transfers) != 0 {
					t.Fatalf("unproved history advanced: %+v, %v", batch, err)
				}
				return
			}
			if err != nil || len(batch.Transfers) != 1 || !batch.CaughtUp || batch.Transfers[0].Currency != "USDC" || batch.Transfers[0].Amount != "1.000000" || !batch.Transfers[0].Verified {
				t.Fatalf("versioned transfer = %+v, %v", batch, err)
			}
			var next solCursor
			if err := json.Unmarshal([]byte(batch.Cursor.Value), &next); err != nil || next.Accounts[account].Last != signature {
				t.Fatalf("versioned cursor did not recover: %+v, %v", next, err)
			}
		})
	}
}
