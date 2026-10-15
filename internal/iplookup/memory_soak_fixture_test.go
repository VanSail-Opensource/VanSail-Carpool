package iplookup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"github.com/gorilla/websocket"
)

const soakRecipient = "0x1111111111111111111111111111111111111111"

func soakHash(n uint64) string { return fmt.Sprintf("0x%064x", n) }
func soakBlock(n uint64) map[string]any {
	return map[string]any{"number": fmt.Sprintf("0x%x", n), "hash": soakHash(n), "timestamp": fmt.Sprintf("0x%x", time.Now().Add(-time.Minute).Unix())}
}

func soakLogs(spec usdt.NetworkSpec, n uint64) []map[string]any {
	logs := make([]map[string]any, 2000)
	for i := range logs {
		logs[i] = map[string]any{"address": spec.Token, "topics": []string{
			"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
			soakHash(0), "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(soakRecipient, "0x"),
		}, "data": soakHash(1000000), "blockNumber": fmt.Sprintf("0x%x", n), "blockHash": soakHash(n), "transactionHash": soakHash(n), "logIndex": fmt.Sprintf("0x%x", i), "removed": false}
	}
	return logs
}

func soakRPCServer(t *testing.T) *httptest.Server {
	t.Helper()
	var heads atomic.Uint64
	heads.Store(2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/fail/") {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ws/") {
			soakSocket(w, r)
			return
		}
		spec, ok := usdt.Network(strings.TrimPrefix(r.URL.Path, "/"))
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var request struct {
			ID     uint64
			Method string
			Params []json.RawMessage
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			return
		}
		var result any
		switch request.Method {
		case "eth_chainId":
			result = fmt.Sprintf("0x%x", spec.ChainID)
		case "eth_call":
			result = fmt.Sprintf("0x%x", spec.Decimals)
		case "eth_getFinalizedBlock":
			result = soakBlock(heads.Add(1))
		case "eth_getBlockByNumber":
			var number string
			_ = json.Unmarshal(request.Params[0], &number)
			n, err := strconv.ParseUint(strings.TrimPrefix(number, "0x"), 16, 64)
			if err != nil {
				n = heads.Add(1)
			}
			result = soakBlock(n)
		case "eth_getLogs":
			var query struct{ FromBlock string }
			_ = json.Unmarshal(request.Params[0], &query)
			n, _ := strconv.ParseUint(strings.TrimPrefix(query.FromBlock, "0x"), 16, 64)
			result = soakLogs(spec, n)
		case "eth_getTransactionReceipt":
			var hash string
			_ = json.Unmarshal(request.Params[0], &hash)
			n, _ := strconv.ParseUint(strings.TrimPrefix(hash, "0x"), 16, 64)
			result = map[string]any{"status": "0x1", "blockHash": hash, "transactionHash": hash, "logs": soakLogs(spec, n)}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": request.ID, "result": result})
	}))
	t.Cleanup(server.Close)
	return server
}

func soakSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
	if err != nil {
		return
	}
	var write sync.Mutex
	stop := make(chan struct{})
	var work sync.WaitGroup
	defer func() { close(stop); _ = conn.Close(); work.Wait() }()
	work.Add(1)
	go func() {
		defer work.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				write.Lock()
				err := conn.WriteJSON(map[string]any{"method": "eth_subscription"})
				write.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()
	for {
		var request struct{ ID uint64 }
		if conn.ReadJSON(&request) != nil {
			return
		}
		write.Lock()
		err := conn.WriteJSON(map[string]any{"id": request.ID, "result": "subscription"})
		write.Unlock()
		if err != nil {
			return
		}
	}
}
