package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHTTPRateLimitsArePersistedAndErrorsAreRedacted(t *testing.T) {
	t.Parallel()
	c, repo := testClient(t, "ethereum", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "120"); w.WriteHeader(429) }))
	var result string
	err := c.rpc(context.Background(), "eth_chainId", []any{}, &result)
	var pause *Pause
	if !errors.As(err, &pause) || ErrorCode(err) != "USDT_RPC_RATE_LIMITED" || time.Until(repo.lastFailure) < time.Minute {
		t.Fatalf("rate limit=%v cooldown=%v", err, repo.lastFailure)
	}
	if ErrorCode(errors.New("https://host.example/secret")) != "USDT_STORAGE_UNAVAILABLE" {
		t.Fatal("raw URL exposed")
	}
	if until := retryAfter("invalid"); time.Until(until) < 20*time.Second {
		t.Fatal("missing fallback cooldown")
	}
	if until := retryAfter(time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)); time.Until(until) < 30*time.Minute {
		t.Fatal("HTTP date ignored")
	}
}

func TestWebSocketAndHTTPShareAdmissionAndReconnect(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	upgrader := websocket.Upgrader{}
	c, repo := testClient(t, "ethereum", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	repo.mu.Lock()
	repo.limit = repo.admissions
	repo.mu.Unlock()
	before := requests.Load()
	if err := c.rpc(context.Background(), "eth_chainId", []any{}, &result); ErrorCode(err) != "USDT_RPC_QUOTA_EXHAUSTED" {
		t.Fatalf("shared budget bypassed: %v", err)
	}
	if requests.Load() != before {
		t.Fatal("traffic sent after exhaustion")
	}
	repo.mu.Lock()
	repo.limit = 0
	repo.mu.Unlock()
	c.close()
	if err := c.subscribe(context.Background(), []string{"0x1111111111111111111111111111111111111111"}); err != nil {
		t.Fatal("reconnect", err)
	}
	c.endpoint.RPCURL = c.endpoint.WSURL
	if err := c.rpc(context.Background(), "eth_chainId", []any{}, &result); err != nil || result != "0x1" {
		t.Fatal("WS-only RPC", result, err)
	}
}
