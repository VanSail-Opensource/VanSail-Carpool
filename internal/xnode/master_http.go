package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func (m *Master) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	lifetime := m.ctx
	m.mu.RUnlock()
	if lifetime == nil || lifetime.Err() != nil {
		http.Error(w, "agent controller unavailable", http.StatusServiceUnavailable)
		return
	}
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(raw) != 43 {
		http.Error(w, "agent authentication required", http.StatusUnauthorized)
		return
	}
	hash := sha256.Sum256([]byte(raw))
	node, err := m.service.repo.XNodeNodeByToken(r.Context(), hash[:])
	if err != nil {
		http.Error(w, "agent authentication required", http.StatusUnauthorized)
		return
	}
	protocolOK := false
	for _, protocol := range websocket.Subprotocols(r) {
		if protocol == "x-node.v1" {
			protocolOK = true
		}
	}
	if !websocket.IsWebSocketUpgrade(r) || !protocolOK {
		w.Header().Set("Sec-WebSocket-Protocol", "x-node.v1")
		http.Error(w, "agent WebSocket upgrade required", http.StatusUpgradeRequired)
		return
	}
	upgrader := websocket.Upgrader{Subprotocols: []string{"x-node.v1"}, CheckOrigin: func(*http.Request) bool { return true }, HandshakeTimeout: 15 * time.Second}
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer socket.Close()
	socket.SetReadLimit(wire.MaxFrameBytes)
	if err = socket.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return
	}
	kind, data, err := socket.ReadMessage()
	if err != nil || kind != websocket.TextMessage {
		return
	}
	message, err := wire.Parse(data)
	if err != nil || message.Type != "agent.hello" {
		return
	}
	var hello wire.AgentHello
	if err = wire.Decode(message.Data, &hello); err != nil || len(hello.AgentID) != 32 || len(hello.BootID) != 32 {
		return
	}
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "xnode-" + node.ID, Capacity: 128})
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(lifetime)
	defer cancel()
	if err = queue.Start(ctx); err != nil {
		return
	}
	defer queue.Shutdown(context.Background())
	peer := &agentPeer{nodeID: node.ID, agentID: hello.AgentID, bootID: hello.BootID, socket: socket, queue: queue, ctx: ctx, cancel: cancel, revision: message.Revision, pending: map[string]chan wire.Message{}, lastControls: map[int]bool{}, pong: make(chan string, 8)}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && r.Header.Get("X-Forwarded-For") == "" {
		peer.publicIP, _ = FirewallPublicIP(host)
	}
	m.mu.Lock()
	if lifetime.Err() != nil {
		m.mu.Unlock()
		return
	}
	// Enrollment rotation uses the same lock. Recheck after the hello so a token
	// revoked during the handshake cannot publish a new authenticated peer.
	current, authErr := m.service.repo.XNodeNodeByToken(ctx, hash[:])
	if authErr != nil || current.ID != node.ID {
		m.mu.Unlock()
		return
	}
	if err = m.service.repo.BindXNodeAgent(ctx, node.ID, hello.AgentID); err != nil {
		m.mu.Unlock()
		return
	}
	if previous := m.peers[node.ID]; previous != nil {
		previous.cancel()
		previous.socket.Close()
	}
	m.peers[node.ID] = peer
	m.live[node.ID] = liveSnapshot{hello: hello}
	m.wg.Add(1)
	m.mu.Unlock()
	m.rememberNodeIPs(peer, json.RawMessage(`{}`))
	defer func() {
		m.mu.Lock()
		if m.peers[node.ID] == peer {
			delete(m.peers, node.ID)
			snapshot := m.live[node.ID]
			snapshot.receivedAt = time.Time{}
			m.live[node.ID] = snapshot
		}
		m.mu.Unlock()
		m.wg.Done()
	}()
	if err = socket.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
		return
	}
	socket.SetPongHandler(func(payload string) error {
		select {
		case peer.pong <- payload:
		default:
		}
		return socket.SetReadDeadline(time.Now().Add(90 * time.Second))
	})
	socket.SetPingHandler(func(payload string) error {
		return upstreamqueue.Execute(ctx, queue, func(context.Context) error {
			return socket.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(10*time.Second))
		})
	})
	receivers := m.startPeerReceivers(peer)
	defer receivers.stop()
	for {
		kind, data, err = socket.ReadMessage()
		if err != nil {
			return
		}
		if kind != websocket.TextMessage {
			return
		}
		if err = socket.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return
		}
		message, err = wire.Parse(data)
		if err != nil {
			return
		}
		if err = receivers.offer(message); err != nil {
			return
		}
	}
}
