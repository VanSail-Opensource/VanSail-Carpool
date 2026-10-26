package xnode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

// The fake peer exercises actual serialized WebSocket grants and the receiving
// validation boundary; it never runs an external process or modifies a node.
func computeNodeFixture(t *testing.T, m *Master, id string, delay time.Duration, disconnect bool) *atomic.Int32 {
	t.Helper()
	ctx, cancel := context.WithCancel(m.ctx)
	sockets := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		s, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		sockets <- s
		<-ctx.Done()
		s.Close()
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		cancel()
		server.Close()
		t.Fatal(err)
	}
	socket := <-sockets
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "compute-fixture-" + id, Capacity: 128})
	if err != nil {
		t.Fatal(err)
	}
	if err = queue.Start(ctx); err != nil {
		t.Fatal(err)
	}
	peer := &agentPeer{nodeID: id, ctx: ctx, cancel: cancel, socket: socket, queue: queue, computePool: true}
	m.mu.Lock()
	m.peers[id] = peer
	m.live[id] = liveSnapshot{receivedAt: time.Now(), report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	m.mu.Unlock()
	grants := &atomic.Int32{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var message wire.Message
			if client.ReadJSON(&message) != nil {
				return
			}
			switch message.Type {
			case "compute.announce":
				var offer wire.ComputeAnnouncement
				if wire.Decode(message.Data, &offer) != nil {
					return
				}
				if delay > 0 {
					timer := time.NewTimer(delay)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
				_ = m.receiveCompute(ctx, peer, wire.New("compute.claim", offer.Signal.BatchID, 0, offer.Signal))
			case "compute.grant":
				grants.Add(1)
				if disconnect {
					cancel()
					return
				}
				var g wire.ComputeGrant
				var b compute.Batch
				if wire.Decode(message.Data, &g) != nil || json.Unmarshal(g.Input, &b) != nil {
					return
				}
				out, err := compute.Run(ctx, b)
				if err != nil {
					return
				}
				raw, err := compute.Encode(out)
				if err != nil {
					return
				}
				total := (len(raw) + compute.ChunkBytes - 1) / compute.ChunkBytes
				for i := 0; i < total; i++ {
					chunk := wire.ComputeChunk{Signal: g.Offer.Signal, ResultHash: compute.Digest(raw), Index: i, Total: total, Data: base64.StdEncoding.EncodeToString(raw[i*compute.ChunkBytes : min((i+1)*compute.ChunkBytes, len(raw))])}
					_ = m.receiveCompute(ctx, peer, wire.New("compute.result", g.Offer.Signal.BatchID, 0, chunk))
				}
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		client.Close()
		socket.Close()
		<-done
		queue.Shutdown(context.Background())
		server.Close()
	})
	return grants
}
