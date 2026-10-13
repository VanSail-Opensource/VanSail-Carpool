package usdt

import (
	"context"
	"encoding/base64"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

func tonProofFixture() (tonTransfer, tonTrace, string, string) {
	owner := "0:" + strings.Repeat("1", 64)
	wallet := "0:" + strings.Repeat("2", 64)
	sender := "0:" + strings.Repeat("3", 64)
	sourceWallet := "0:" + strings.Repeat("4", 64)
	body := cell.BeginCell().MustStoreUInt(0x178d4519, 32).MustStoreUInt(7, 64).MustStoreBigCoins(big.NewInt(1000000)).MustStoreAddr(address.MustParseRawAddr(sender)).EndCell().ToBOC()
	item := tonTransfer{Amount: "1000000", Destination: owner, Source: sender, SourceWallet: sourceWallet, TxHash: "sender-hash", TraceID: "trace", QueryID: "7"}
	spec, _ := Network("ton")
	item.Master = spec.Token
	receive := tonTransaction{Account: wallet, Hash: "receive-hash", Now: time.Now().UTC().Add(-time.Minute).Unix(), MCSeqno: 100}
	receive.Description.Compute.Success = true
	receive.In = tonMessage{Hash: "message", Source: sourceWallet, Destination: wallet, Opcode: "0x178d4519"}
	receive.In.Content.Body = base64.StdEncoding.EncodeToString(body)
	send := tonTransaction{Account: sourceWallet, Hash: item.TxHash}
	send.Description.Compute.Success = true
	send.Out = []tonMessage{{Hash: "message", Destination: wallet}}
	trace := tonTrace{ID: "trace", MCSeqno: "100", Transactions: map[string]tonTransaction{"sender": send, "receiver": receive}}
	trace.Info.State = "complete"
	return item, trace, wallet, owner
}

func TestTONVerifiesRecipientWalletBodyAndMasterchainFinality(t *testing.T) {
	t.Parallel()
	spec, _ := Network("ton")
	item, trace, wallet, owner := tonProofFixture()
	c, _ := testClient(t, "ton", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/masterchainInfo":
			sendJSON(w, map[string]any{"last": map[string]any{"global_id": -239, "workchain": -1}})
		case "/jetton/masters":
			sendJSON(w, map[string]any{"jetton_masters": []any{map[string]any{"address": spec.Token, "jetton_content": map[string]any{"decimals": "6"}}}})
		case "/jetton/transfers":
			if r.URL.Query().Get("direction") != "in" || r.URL.Query().Get("jetton_master") != spec.Token {
				t.Error("unfiltered TON scan")
			}
			sendJSON(w, map[string]any{"jetton_transfers": []tonTransfer{item}})
		case "/jetton/wallets":
			sendJSON(w, map[string]any{"jetton_wallets": []any{map[string]any{"address": wallet, "owner": owner, "jetton": spec.Token}}})
		case "/traces":
			sendJSON(w, map[string]any{"traces": []tonTrace{trace}})
		default:
			t.Error("unexpected TON route")
			w.WriteHeader(400)
		}
	}))
	reader := &scanner{client: c}
	batch, err := reader.scan(context.Background(), Cursor{}, []string{owner})
	if err != nil || len(batch.Transfers) != 1 || batch.Transfers[0].TxHash != "receive-hash" || batch.Transfers[0].Recipient != owner || !batch.CaughtUp {
		t.Fatalf("batch=%+v %v", batch, err)
	}
	for _, change := range []func(*tonTrace){func(t *tonTrace) { t.Incomplete = true }, func(t *tonTrace) { t.MCSeqno = "0" }, func(t *tonTrace) {
		r := t.Transactions["receiver"]
		r.Description.Aborted = true
		t.Transactions["receiver"] = r
	}, func(t *tonTrace) {
		r := t.Transactions["receiver"]
		r.In.Content.Body = "invalid"
		t.Transactions["receiver"] = r
	}} {
		_, bad, _, _ := tonProofFixture()
		change(&bad)
		if _, err := decodeTONTransfer(spec, item, bad, wallet, owner); err == nil {
			t.Fatal("invalid TON proof accepted")
		}
	}
}
