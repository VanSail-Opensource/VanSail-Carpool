package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

func TestHTTPRateLimitedResponseDoesNotCreateCooldown(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	c, _ := testClient(t, "ethereum", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(429) }))
	var result string
	for i := 0; i < 2; i++ {
		if err := c.rpc(context.Background(), "eth_chainId", []any{}, &result); ErrorCode(err) != ErrRPC.Error() {
			t.Fatalf("rate-limited request=%v", err)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("requests=%d; expected immediate retry without cooldown", requests.Load())
	}
	if ErrorCode(errors.New("https://host.example/secret")) != "USDT_STORAGE_UNAVAILABLE" {
		t.Fatal("raw URL exposed")
	}
}

func TestWebSocketAndHTTPRequestsReconnect(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	upgrader := websocket.Upgrader{}
	c, _ := testClient(t, "ethereum", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			for {
				var req struct {
					ID     uint64 `json:"id"`
					Method string `json:"method"`
				}
				if conn.ReadJSON(&req) != nil {
					return
				}
				result := "0x1"
				if req.Method == "eth_subscribe" {
					result = "subscription"
				}
				if conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}) != nil {
					return
				}
			}
		}
		var req struct {
			ID uint64 `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		sendJSON(w, map[string]any{"id": req.ID, "result": "0x1"})
	}))
	c.endpoint.WSURL = "ws" + strings.TrimPrefix(c.endpoint.RPCURL, "http")
	c.endpoint.UseWS = true
	if err := c.subscribe(context.Background(), []string{"0x1111111111111111111111111111111111111111"}); err != nil {
		t.Fatal(err)
	}
	var result string
	if err := c.rpc(context.Background(), "eth_chainId", []any{}, &result); err != nil {
		t.Fatal(err)
	}
	if err := c.rpc(context.Background(), "eth_chainId", []any{}, &result); err != nil {
		t.Fatal("HTTP request after WS subscription", err)
	}
	c.close()
	if err := c.subscribe(context.Background(), []string{"0x1111111111111111111111111111111111111111"}); err != nil {
		t.Fatal("reconnect", err)
	}
	c.endpoint.RPCURL = c.endpoint.WSURL
	if err := c.rpc(context.Background(), "eth_chainId", []any{}, &result); err != nil || result != "0x1" {
		t.Fatal("WS-only RPC", result, err)
	}
}
