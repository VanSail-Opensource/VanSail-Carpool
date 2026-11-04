package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func TestAutomaticLeaseSocketBatchesReplayAndEnableFence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	token := strings.Repeat("a", 43)
	m := NewMaster(NewService(reportSocketRepository{token: sha256.Sum256([]byte(token))}, nil, nil, nil, nil), nil)
	m.ctx = ctx
	server := httptest.NewServer(m)
	socket, _, err := (&websocket.Dialer{Subprotocols: []string{"x-node.v1"}}).Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		cancel()
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); socket.Close(); server.Close(); m.Wait() })
	hello := wire.AgentHello{AgentID: strings.Repeat("1", 32), BootID: strings.Repeat("2", 32), Capabilities: []string{wire.AutomaticLeaseCapability}}
	if err := socket.WriteJSON(wire.New("agent.hello", "", 0, hello)); err != nil {
		t.Fatal(err)
	}
	var peer *agentPeer
	for peer == nil && ctx.Err() == nil {
		m.mu.RLock()
		peer = m.peers["node"]
		m.mu.RUnlock()
		if peer == nil {
			time.Sleep(time.Millisecond)
		}
	}
	if peer == nil {
		t.Fatal("authenticated peer did not register")
	}
	setFirewall := func(enabled bool, revision uint64) {
		m.recordWhitelistState(peer, wire.Config{Firewall: wire.FirewallPolicy{Enabled: enabled}}, revision)
		m.mu.Lock()
		snapshot := m.live["node"]
		snapshot.report = wire.TelemetryReport{Ready: true, Synchronized: true, ConfigRevision: revision}
		snapshot.receivedAt = m.service.Now()
		m.live["node"] = snapshot
		m.mu.Unlock()
	}
	setFirewall(true, 1)
	at := time.Now().UTC()
	m.automaticLeases.observe(automaticLeaseFixture(600, at), at, time.Minute)
	batches := make(chan wire.AutomaticLeaseUpdate, 16)
	hold := make(chan chan struct{}, 1)
	blocked := make(chan struct{}, 1)
	readErrors := make(chan error, 1)
	go func() {
		for {
			var message wire.Message
			if err := socket.ReadJSON(&message); err != nil {
				return
			}
			if message.Type != "command" {
				continue
			}
			var command struct {
				Name string          `json:"name"`
				Args json.RawMessage `json:"args"`
			}
			var update wire.AutomaticLeaseUpdate
			if err := wire.Decode(message.Data, &command); err != nil {
				readErrors <- err
				return
			}
			if err := wire.Decode(command.Args, &update); err != nil {
				readErrors <- err
				return
			}
			batches <- update
			select {
			case wait := <-hold:
				blocked <- struct{}{}
				select {
				case <-wait:
				case <-ctx.Done():
					return
				}
			default:
			}
			ack := wire.AutomaticLeaseAck{Epoch: update.Epoch, Generation: update.Leases[len(update.Leases)-1].Generation}
			if err := socket.WriteJSON(wire.New("command.result", message.ID, 0, map[string]any{"id": message.ID, "command": command.Name, "status": "completed", "data": ack})); err != nil {
				readErrors <- err
				return
			}
		}
	}()
	checkDelivery := func() {
		t.Helper()
		if more, err := m.deliverAutomaticLeases(ctx, peer); err != nil || more {
			t.Fatal("bounded delivery failed", more, err)
		}
		seen := map[string]bool{}
		for range 3 {
			batch := <-batches
			if len(batch.Leases) > wire.MaxAutomaticLeaseBatch {
				t.Fatal("delivery exceeded batch bound")
			}
			for _, grant := range batch.Leases {
				if seen[grant.IP] {
					t.Fatal("duplicate grant in one synchronization")
				}
				seen[grant.IP] = true
			}
		}
		if len(seen) != 600 {
			t.Fatal("missing leases", len(seen))
		}
	}
	checkDelivery()
	if _, err := m.deliverAutomaticLeases(ctx, peer); err != nil || len(batches) != 0 {
		t.Fatal("steady reports delivered unchanged grants", err)
	}
	setFirewall(false, 2)
	setFirewall(true, 3)
	wait := make(chan struct{})
	hold <- wait
	finished := make(chan error, 1)
	go func() { _, err := m.deliverAutomaticLeases(ctx, peer); finished <- err }()
	select {
	case <-blocked:
	case err := <-readErrors:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("lease command was not sent")
	}
	setFirewall(false, 4)
	setFirewall(true, 5)
	close(wait)
	if err := <-finished; err != ErrConflict {
		t.Fatal("stale acknowledgement crossed a firewall enable boundary", err)
	}
	<-batches
	checkDelivery()
}
