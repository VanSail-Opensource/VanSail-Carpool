package usdt

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func solanaProofFixture(spec NetworkSpec, owner, account, signature string) solTransaction {
	var tx solTransaction
	raw, _ := json.Marshal(map[string]any{"slot": 101, "blockTime": time.Now().Add(-time.Minute).Unix(), "transaction": map[string]any{"signatures": []string{signature}, "message": map[string]any{"accountKeys": []any{map[string]string{"pubkey": account}}, "instructions": []any{map[string]any{"programId": solanaTokenProgram, "parsed": map[string]any{"type": "transferChecked", "info": map[string]any{"destination": account, "mint": spec.Token, "tokenAmount": map[string]any{"amount": "1000000", "decimals": 6}}}}}}}, "meta": map[string]any{"err": nil, "preTokenBalances": []any{}, "postTokenBalances": []any{map[string]any{"accountIndex": 0, "mint": spec.Token, "owner": owner, "programId": solanaTokenProgram, "uiTokenAmount": map[string]any{"decimals": 6}}}, "innerInstructions": []any{}}})
	_ = json.Unmarshal(raw, &tx)
	return tx
}

func TestSolanaRecoversHistoryWithoutWebSocketNotifications(t *testing.T) {
	t.Parallel()
	spec, _ := Network("solana")
	owner := spec.Token
	account, err := associatedTokenAddress(owner, spec.Token)
	if err != nil {
		t.Fatal(err)
	}
	signature := "receiving-signature"
	tx := solanaProofFixture(spec, owner, account, signature)
	c, _ := testClient(t, "solana", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		var result any
		switch request.Method {
		case "getGenesisHash":
			result = "5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d"
		case "getTokenSupply":
			result = map[string]any{"value": map[string]any{"decimals": 6}}
		case "getSlot":
			result = 100
		case "getTokenAccountsByOwner":
			result = map[string]any{"value": []any{}}
		case "getSignaturesForAddress":
			result = []any{map[string]any{"signature": signature, "slot": 101, "err": nil, "confirmationStatus": "finalized"}}
		case "getTransaction":
			result = tx
		default:
			t.Errorf("unexpected Solana method %s", request.Method)
		}
		sendJSON(w, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	reader := &scanner{client: c}
	batch, err := reader.scan(context.Background(), Cursor{}, []string{owner})
	if err != nil || len(batch.Transfers) != 1 || batch.Transfers[0].Recipient != owner || batch.Transfers[0].Amount != "1.000000" || !batch.CaughtUp {
		t.Fatalf("batch=%+v %v", batch, err)
	}
	bad := tx
	bad.Slot = 102
	if _, err := decodeSolanaTransaction(spec, bad, signature, 101, []string{owner}); err == nil {
		t.Fatal("wrong slot accepted")
	}
	bad = tx
	bad.Meta.Err = json.RawMessage(`{"InstructionError":[0,"failed"]}`)
	if _, err := decodeSolanaTransaction(spec, bad, signature, 101, []string{owner}); err == nil {
		t.Fatal("failed transaction accepted")
	}
	if _, err := associatedTokenAddress("invalid", spec.Token); err == nil {
		t.Fatal("invalid owner accepted")
	}
}
