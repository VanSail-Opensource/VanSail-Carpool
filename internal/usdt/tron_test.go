package usdt

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTronConfirmedHistoryUsesSolidReceiptEvents(t *testing.T) {
	t.Parallel()
	spec, _ := Network("tron")
	wallet, err := tronHexAddress(strings.Repeat("11", 20))
	if err != nil {
		t.Fatal(err)
	}
	token, _ := decodeBase58(spec.Token)
	hash := strings.Repeat("a", 64)
	r := tronReceipt{ID: hash, BlockNumber: 100, Timestamp: time.Now().UTC().Add(-time.Minute).UnixMilli()}
	r.Receipt.Result = "SUCCESS"
	r.Logs = append(r.Logs, struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	}{Address: hex.EncodeToString(token[1:21]), Topics: []string{strings.TrimPrefix(transferTopic, "0x"), strings.Repeat("0", 64), strings.Repeat("0", 24) + strings.Repeat("11", 20)}, Data: strings.Repeat("0", 58) + "0f4240"})
	c, _ := testClient(t, "tron", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/wallet/getblockbynum":
			sendJSON(w, map[string]any{"blockID": "00000000000000001ebf88508a03865c71d452e25f4d51194196a1d22b6653dc"})
		case "/walletsolidity/triggerconstantcontract":
			sendJSON(w, map[string]any{"result": map[string]any{"result": true}, "constant_result": []string{"06"}})
		case "/walletsolidity/gettransactioninfobyid":
			sendJSON(w, r)
		default:
			if req.URL.Query().Get("only_confirmed") != "true" || req.URL.Query().Get("contract_address") != spec.Token {
				t.Error("history not filtered")
			}
			sendJSON(w, map[string]any{"success": true, "data": []any{map[string]any{"transaction_id": hash, "type": "Transfer", "token_info": map[string]any{"address": spec.Token, "decimals": 6}}}, "meta": map[string]any{}})
		}
	}))
	reader := &scanner{client: c}
	if err := reader.check(context.Background()); err != nil {
		t.Fatal(err)
	}
	batch, err := reader.scan(context.Background(), Cursor{}, []string{wallet})
	if err != nil || len(batch.Transfers) != 1 || batch.Transfers[0].Amount != "1.000000" || !batch.CaughtUp {
		t.Fatalf("batch=%+v %v", batch, err)
	}
	r.Receipt.Result = "REVERT"
	transfers, err := decodeTronReceipt(spec, r, hash, []string{wallet})
	if err != nil || len(transfers) != 0 {
		t.Fatal("failed TRON receipt credited")
	}
	r.ID = "wrong"
	if _, err := decodeTronReceipt(spec, r, hash, []string{wallet}); err == nil {
		t.Fatal("wrong receipt identity accepted")
	}
}
