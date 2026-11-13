package usdt

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestIndexedHistoryRequiresAnExplicitPage(t *testing.T) {
	t.Parallel()
	for _, network := range []string{"aptos", "tron", "ton"} {
		for _, shape := range []string{"missing", "null", "empty"} {
			t.Run(network+"/"+shape, func(t *testing.T) {
				page := map[string]any{}
				key := "jetton_transfers"
				if network == "aptos" {
					key = "fungible_asset_activities"
				} else if network == "tron" {
					key = "data"
					page["success"] = true
				}
				if shape == "null" {
					page[key] = nil
				} else if shape == "empty" {
					page[key] = []any{}
				}
				var response any = page
				if network == "aptos" {
					response = map[string]any{"data": page}
				}
				client, _ := testClient(t, network, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					sendJSON(w, response)
				}))
				cursor := Cursor{Value: cursorJSON(timePage{Start: 1, End: 2})}
				if network == "aptos" {
					cursor.Value = cursorJSON(aptosCursor{Version: "100", Index: 0})
				}
				batch, err := (&scanner{client: client, checked: true}).scan(context.Background(), cursor, []string{client.spec.Token})
				if shape == "empty" {
					if err != nil || batch.Cursor.Value == "" {
						t.Fatal("explicit empty page rejected", batch, err)
					}
				} else if err == nil || batch.Cursor.Value != "" || len(batch.Transfers) != 0 {
					t.Fatal("unavailable history advanced cursor", batch, err)
				}
			})
		}
	}
}

func TestSolanaAccountDiscoveryRequiresAnExplicitPage(t *testing.T) {
	t.Parallel()
	for _, shape := range []string{"missing", "null", "empty"} {
		t.Run(shape, func(t *testing.T) {
			result := map[string]any{}
			if shape == "null" {
				result["value"] = nil
			} else if shape == "empty" {
				result["value"] = []any{}
			}
			client, _ := testClient(t, "solana", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID uint64 `json:"id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				sendJSON(w, map[string]any{"id": request.ID, "result": result})
			}))
			accounts, err := (&scanner{client: client}).solanaAccounts(context.Background(), []string{client.spec.Token})
			if shape == "empty" {
				if err != nil || len(accounts) != 1 {
					t.Fatal("explicit empty discovery lost deterministic ATA", accounts, err)
				}
			} else if err == nil || accounts != nil {
				t.Fatal("unknown discovery treated as no token accounts", accounts, err)
			}
		})
	}
}
