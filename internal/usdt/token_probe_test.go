package usdt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestQueuedUSDCProbeChecksPinnedEVMIdentityAndPrecision(t *testing.T) {
	for _, spec := range Registry {
		if spec.Currency() != "USDC" || spec.Family != "evm" {
			continue
		}
		for _, precision := range []int{6, 18} {
			t.Run(fmt.Sprintf("%s/%d", spec.Network, precision), func(t *testing.T) {
				c, _ := testClient(t, spec.Rail, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var request struct {
						ID     int               `json:"id"`
						Method string            `json:"method"`
						Params []json.RawMessage `json:"params"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					var result any
					switch request.Method {
					case "eth_chainId":
						result = fmt.Sprintf("0x%x", spec.ChainID)
					case "eth_call":
						var call struct {
							To   string `json:"to"`
							Data string `json:"data"`
						}
						if len(request.Params) != 2 || json.Unmarshal(request.Params[0], &call) != nil || call.To != spec.Token || call.Data != "0x313ce567" {
							t.Error("precision probe did not use pinned contract")
						}
						result = fmt.Sprintf("0x%x", precision)
					case "eth_getBlockByNumber":
						result = map[string]string{"number": "0x64", "hash": "0x" + strings.Repeat("a", 64)}
					default:
						t.Errorf("unexpected probe %s", request.Method)
					}
					sendJSON(w, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
				}))
				err := (&scanner{client: c}).check(context.Background())
				if (err == nil) != (precision == 6) {
					t.Fatal("precision mismatch admitted", precision, err)
				}
			})
		}
	}
}
