package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEVMScannersUsePinnedTokensReceiptsAndFinalizedRanges(t *testing.T) {
	t.Parallel()
	for _, network := range []string{"ethereum", "bsc", "polygon", "arbitrum", "xlayer", "plasma"} {
		t.Run(network, func(t *testing.T) {
			spec, _ := Network(network)
			wallet := "0x" + strings.Repeat("1", 40)
			atom, _ := Atomic("1.001", spec.Decimals)
			block := func(n uint64) evmBlock {
				return evmBlock{Number: toHex(n), Hash: fmt.Sprintf("0x%064x", n+1000), Timestamp: toHex(uint64(time.Now().UTC().Add(-time.Minute).Unix()))}
			}
			log := evmLog{Address: spec.Token, Topics: []string{transferTopic, evmTopic(wallet), evmTopic(wallet)}, Data: fmt.Sprintf("0x%064x", atom), BlockNumber: "0x64", BlockHash: block(100).Hash, TxHash: "0x" + strings.Repeat("a", 64), Index: "0x0"}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     uint64            `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if json.NewDecoder(r.Body).Decode(&req) != nil {
					t.Error("invalid RPC JSON")
					w.WriteHeader(400)
					return
				}
				var result any
				switch req.Method {
				case "eth_chainId":
					result = toHex(uint64(spec.ChainID))
				case "eth_call":
					result = fmt.Sprintf("0x%064x", spec.Decimals)
				case "eth_getFinalizedBlock":
					result = block(100)
				case "eth_getBlockByNumber":
					var number string
					_ = json.Unmarshal(req.Params[0], &number)
					if number == "finalized" {
						result = block(100)
					} else {
						n, _ := hexNumber(number)
						result = block(n)
					}
				case "eth_getLogs":
					var filter struct {
						Address string `json:"address"`
						From    string `json:"fromBlock"`
						To      string `json:"toBlock"`
					}
					_ = json.Unmarshal(req.Params[0], &filter)
					if filter.Address != spec.Token || filter.From != "0x64" || filter.To != "0x64" {
						t.Errorf("unbounded or unfiltered scan: %+v", filter)
					}
					result = []evmLog{log}
				case "eth_getTransactionReceipt":
					result = evmReceipt{Status: "0x1", BlockHash: log.BlockHash, TxHash: log.TxHash, Logs: []evmLog{log}}
				default:
					t.Errorf("unexpected RPC %s", req.Method)
					w.WriteHeader(400)
					return
				}
				sendJSON(w, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			})
			c, _ := testClient(t, network, handler)
			reader := &scanner{client: c}
			ctx := context.Background()
			if err := reader.check(ctx); err != nil {
				t.Fatal(err)
			}
			initial, err := reader.scan(ctx, Cursor{}, []string{wallet})
			if err != nil || initial.Cursor.Value != "100" || !initial.CaughtUp {
				t.Fatalf("initial=%+v %v", initial, err)
			}
			batch, err := reader.scan(ctx, Cursor{Value: "99", Hash: block(99).Hash}, []string{wallet})
			if err != nil || len(batch.Transfers) != 1 || batch.Transfers[0].Amount != FormatAtomic(atom, spec.Decimals) || !batch.CaughtUp {
				t.Fatalf("scan=%+v %v", batch, err)
			}
			if batch.Transfers[0].EventID != strconv.Itoa(0) || !batch.Transfers[0].Finalized {
				t.Fatal("missing event identity or finality")
			}
			if _, err := reader.scan(ctx, Cursor{Value: "99", Hash: "reorg"}, []string{wallet}); !errors.Is(err, ErrProof) {
				t.Fatalf("reorg accepted: %v", err)
			}
			bad := log
			bad.Removed = true
			if _, err := decodeEVMLog(spec, bad, block(100)); err == nil {
				t.Fatal("removed event accepted")
			}
		})
	}
}

func TestEVMRejectsWrongChainAndReceipt(t *testing.T) {
	t.Parallel()
	c, _ := testClient(t, "ethereum", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID uint64 `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		sendJSON(w, map[string]any{"id": request.ID, "result": "0x38"})
	}))
	if err := (&scanner{client: c}).check(context.Background()); !errors.Is(err, ErrProof) {
		t.Fatalf("wrong chain accepted: %v", err)
	}
	if receiptContains(evmReceipt{Status: "0x0"}, evmLog{}) {
		t.Fatal("failed transaction accepted")
	}
}
