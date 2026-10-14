package xnode

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
	"time"
)

func (m *Master) confirmFirewall(ctx context.Context, peer *agentPeer, message wire.Message) error {
	var pending struct {
		ID            string          `json:"id"`
		Status        string          `json:"status"`
		Deadline      time.Time       `json:"deadline"`
		Unit          string          `json:"unit"`
		Policy        json.RawMessage `json:"policy"`
		DNSRedirect   bool            `json:"dns_redirect"`
		TransactionID string          `json:"transaction_id"`
	}
	if err := json.Unmarshal(message.Data, &pending); err != nil {
		return ErrInvalid
	}
	id := pending.TransactionID
	if id == "" {
		id = pending.ID
	}
	if id == "" {
		return ErrInvalid
	}
	m.mu.Lock()
	snapshot := m.live[peer.nodeID]
	snapshot.firewallPending = id
	m.live[peer.nodeID] = snapshot
	m.mu.Unlock()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		check, cancel := context.WithTimeout(peer.ctx, 15*time.Second)
		defer cancel()
		if err := upstreamqueue.Execute(check, peer.queue, func(context.Context) error {
			return peer.socket.WriteControl(websocket.PingMessage, []byte(id), time.Now().Add(5*time.Second))
		}); err != nil {
			return
		}
		for {
			select {
			case pong := <-peer.pong:
				if pong == id {
					result, err := m.Command(check, peer.nodeID, "firewall-confirm-"+id, "firewall.confirm", map[string]string{"transaction_id": id})
					var reply struct {
						Confirmed bool `json:"confirmed"`
					}
					if err != nil || wire.Decode(result, &reply) != nil || !reply.Confirmed {
						return
					}
					m.mu.Lock()
					current := m.live[peer.nodeID]
					if m.peers[peer.nodeID] == peer && current.firewallPending == id {
						current.firewallPending = ""
						m.live[peer.nodeID] = current
					}
					m.mu.Unlock()
					return
				}
			case <-check.Done():
				return
			}
		}
	}()
	return nil
}
