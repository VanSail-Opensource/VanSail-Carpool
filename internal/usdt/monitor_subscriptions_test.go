package usdt

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

func TestInactiveEndpointSubscriptionsReleaseReadersAndReconnect(t *testing.T) {
	t.Parallel()
	first, firstDials := subscriptionTestClient(t, "ethereum", nil)
	second, _ := subscriptionTestClient(t, "ethereum", nil)
	pool := &subscriptionTestTransport{}
	first.http.Transport = pool
	readers := []*scanner{{client: first}, {client: second}}
	addresses := []string{"0x1111111111111111111111111111111111111111"}
	for _, reader := range readers {
		reader.refreshSubscription(context.Background(), addresses)
		if reader.subscription == "" || reader.subSocket == nil {
			t.Fatal("initial subscription failed")
		}
	}
	retired, selected := readers[0].subSocket, readers[1].subSocket
	closeInactiveSubscriptions(readers, readers[1])
	assertSubscriptionClosed(t, readers[0], retired)
	if pool.closed.Load() != 0 {
		t.Fatal("endpoint switch closed shared HTTP idle connections")
	}
	if readers[1].subSocket != selected || readers[1].subscription == "" {
		t.Fatal("selected subscription was discarded")
	}
	select {
	case <-selected.readDone:
		t.Fatal("selected reader stopped")
	default:
	}
	var chain string
	if err := second.rpc(context.Background(), "eth_chainId", []any{}, &chain); err != nil || chain != "0x1" {
		t.Fatal("selected socket unusable", chain, err)
	}
	readers[0].refreshSubscription(context.Background(), addresses)
	if readers[0].subSocket == nil || readers[0].subSocket == retired || firstDials.Load() != 2 {
		t.Fatal("retired endpoint did not reconnect once")
	}
	reconnected := readers[0].subSocket
	closeInactiveSubscriptions(readers, nil)
	assertSubscriptionClosed(t, readers[0], reconnected)
	assertSubscriptionClosed(t, readers[1], selected)
}

func TestPartialSubscriptionFailureClosesBeforeRetry(t *testing.T) {
	t.Parallel()
	var fail atomic.Bool
	fail.Store(true)
	client, dials := subscriptionTestClient(t, "solana", &fail)
	reader := &scanner{client: client}
	socket, err := client.connection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	addresses := []string{client.spec.Token, solanaTokenProgram}
	reader.refreshSubscription(context.Background(), addresses)
	assertSubscriptionClosed(t, reader, socket)
	fail.Store(false)
	reader.refreshSubscription(context.Background(), addresses)
	if reader.subscription == "" || reader.subSocket == nil || reader.subSocket == socket || dials.Load() != 2 {
		t.Fatal("partial subscription retry did not start a fresh connection")
	}
	current := reader.subSocket
	reader.refreshSubscription(context.Background(), nil)
	assertSubscriptionClosed(t, reader, current)
}

func assertSubscriptionClosed(t *testing.T, reader *scanner, previous *socket) {
	t.Helper()
	if reader.client.ws != nil || reader.subSocket != nil || reader.subscription != "" {
		t.Fatal("inactive endpoint retained socket ownership")
	}
	if previous != nil {
		select {
		case <-previous.readDone:
		default:
			t.Fatal("inactive endpoint reader was not joined")
		}
	}
}

type subscriptionTestTransport struct{ closed atomic.Int64 }

func (t *subscriptionTestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, ErrRPC
}
func (t *subscriptionTestTransport) CloseIdleConnections() { t.closed.Add(1) }

func subscriptionTestClient(t *testing.T, network string, failSecond *atomic.Bool) (*client, *atomic.Int64) {
	t.Helper()
	var dials atomic.Int64
	upgrader := websocket.Upgrader{}
	client, _ := testClient(t, network, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		dials.Add(1)
		defer conn.Close()
		count := 0
		for {
			var request struct {
				ID     uint64 `json:"id"`
				Method string `json:"method"`
			}
			if conn.ReadJSON(&request) != nil {
				return
			}
			count++
			var result any = "0x1"
			if request.Method == "logsSubscribe" {
				result = 7
			} else if request.Method == "eth_subscribe" {
				result = "subscription"
			}
			reply := map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}
			if failSecond != nil && failSecond.Load() && count == 2 {
				delete(reply, "result")
				reply["error"] = map[string]any{"code": -32603, "message": "subscription unavailable"}
			}
			if conn.WriteJSON(reply) != nil {
				return
			}
		}
	}))
	client.endpoint.WSURL = "ws" + strings.TrimPrefix(client.endpoint.RPCURL, "http")
	client.endpoint.RPCURL, client.endpoint.UseWS = client.endpoint.WSURL, true
	return client, &dials
}
