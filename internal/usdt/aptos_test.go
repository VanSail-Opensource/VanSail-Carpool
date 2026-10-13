package usdt

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAptosIndexerCrossChecksCommittedTransactionAndStore(t *testing.T) {
	t.Parallel()
	spec, _ := Network("aptos")
	owner := "0x" + strings.Repeat("1", 64)
	store := "0x" + strings.Repeat("2", 64)
	a := aptosActivity{Version: "101", Index: 0, Owner: owner, Asset: spec.Token, Store: store, Amount: "1000000", Success: true, Type: "0x1::fungible_asset::Deposit"}
	tx := aptosTransaction{Hash: "0x" + strings.Repeat("a", 64), Success: true, Version: "101", Timestamp: quantity(strconv.FormatInt(time.Now().UTC().Add(-time.Minute).UnixMicro(), 10))}
	tx.Events = append(tx.Events, struct {
		Type string `json:"type"`
		Data struct {
			Store  string   `json:"store"`
			Amount quantity `json:"amount"`
		} `json:"data"`
	}{Type: a.Type})
	tx.Events[0].Data.Store = store
	tx.Events[0].Data.Amount = a.Amount
	c, _ := testClient(t, "aptos", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1":
			sendJSON(w, map[string]any{"chain_id": 1})
		case strings.Contains(r.URL.Path, "transactions/by_version"):
			sendJSON(w, tx)
		case strings.Contains(r.URL.Path, "ObjectCore"):
			sendJSON(w, map[string]any{"data": map[string]any{"owner": owner}})
		case strings.Contains(r.URL.Path, "FungibleStore"):
			sendJSON(w, map[string]any{"data": map[string]any{"metadata": map[string]any{"inner": spec.Token}}})
		default:
			var request struct {
				Query string `json:"query"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			if strings.Contains(request.Query, "ledger_infos") {
				sendJSON(w, map[string]any{"data": map[string]any{"ledger_infos": []any{map[string]any{"chain_id": 1}}, "fungible_asset_metadata": []any{map[string]any{"decimals": 6}}}})
			} else if strings.Contains(request.Query, "processor_status") {
				sendJSON(w, map[string]any{"data": map[string]any{"processor_status": []any{map[string]any{"last_success_version": "100"}}}})
			} else {
				sendJSON(w, map[string]any{"data": map[string]any{"fungible_asset_activities": []aptosActivity{a}}})
			}
		}
	}))
	reader := &scanner{client: c}
	ctx := context.Background()
	if err := reader.check(ctx); err != nil {
		t.Fatal(err)
	}
	initial, err := reader.scan(ctx, Cursor{}, []string{owner})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := reader.scan(ctx, initial.Cursor, []string{owner})
	if err != nil || len(batch.Transfers) != 1 || batch.Transfers[0].Amount != "1.000000" {
		t.Fatalf("batch=%+v %v", batch, err)
	}
	tx.Success = false
	if _, err := decodeAptosActivity(spec, a, tx); err == nil {
		t.Fatal("failed Aptos transaction accepted")
	}
}
