package usdt

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestChangedTronRecipientsRestartSameWindowWithoutForeignFingerprint(t *testing.T) {
	t.Parallel()
	first, _ := tronHexAddress(strings.Repeat("11", 20))
	second, _ := tronHexAddress(strings.Repeat("22", 20))
	added, _ := tronHexAddress(strings.Repeat("33", 20))
	old := bindTimePage(timePage{Start: 100, End: 200}, []string{first, second})
	old.Address, old.Fingerprint = 1, "only-valid-for-second"
	client, _ := testClient(t, "tron", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/"+added+"/") || r.URL.Query().Get("fingerprint") != "" || r.URL.Query().Get("min_timestamp") != "100000" || r.URL.Query().Get("max_timestamp") != "200000" {
			t.Error("changed recipients reused another query's pagination or skipped the retained window", r.URL)
		}
		sendJSON(w, map[string]any{"success": true, "data": []any{}})
	}))
	batch, err := (&scanner{client: client, checked: true}).scan(context.Background(), Cursor{Value: cursorJSON(old)}, []string{added, first, second})
	var next timePage
	if err != nil || decodeCursor(batch.Cursor.Value, &next) != nil || next.Start != old.Start || next.End != old.End || next.Address != 1 || next.Recipients == old.Recipients || batch.CaughtUp {
		t.Fatal("recipient change lost the retained history window", next, batch, err)
	}
}

func TestChangedTONRecipientsRestartSameWindowAtFirstPage(t *testing.T) {
	t.Parallel()
	first, second := "0:"+strings.Repeat("1", 64), "0:"+strings.Repeat("2", 64)
	old := bindTimePage(timePage{Start: 100, End: 200}, []string{first})
	old.Offset = 100
	client, _ := testClient(t, "ton", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("offset") != "0" || len(query["owner_address"]) != 2 || query.Get("start_utime") != "100" || query.Get("end_utime") != "200" {
			t.Error("changed recipient query retained a foreign offset", r.URL)
		}
		sendJSON(w, map[string]any{"jetton_transfers": []any{}})
	}))
	batch, err := (&scanner{client: client, checked: true}).scan(context.Background(), Cursor{Value: cursorJSON(old)}, []string{first, second})
	var next timePage
	if err != nil || decodeCursor(batch.Cursor.Value, &next) != nil || next.Completed != 200 {
		t.Fatal("restarted TON window did not preserve its completion boundary", next, err)
	}
	if unchanged := bindTimePage(old, []string{first}); unchanged != old {
		t.Fatal("unchanged query lost its pagination", unchanged, old)
	}
}
