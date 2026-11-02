package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

// Firewall pongs are keyed by transaction, so concurrent confirmations cannot
// consume one another's liveness proof. Duplicate pending reports coalesce.
func (p *agentPeer) watchFirewallPong(id string) (chan struct{}, bool, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.firewallPongs == nil {
		p.firewallPongs = make(map[string]chan struct{})
	}
	if _, exists := p.firewallPongs[id]; exists {
		return nil, false, false
	}
	if len(p.firewallPongs) >= 8 {
		return nil, false, true
	}
	wait := make(chan struct{}, 1)
	p.firewallPongs[id] = wait
	return wait, true, false
}

func (p *agentPeer) firewallPong(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if wait := p.firewallPongs[id]; wait != nil {
		select {
		case wait <- struct{}{}:
		default:
		}
	}
}

func (m *Master) confirmFirewall(ctx context.Context, peer *agentPeer, message wire.Message) error {
	var pending struct {
		ID            string `json:"id"`
		TransactionID string `json:"transaction_id"`
	}
	if err := json.Unmarshal(message.Data, &pending); err != nil {
		return ErrInvalid
	}
	id := pending.TransactionID
	if id == "" {
		id = pending.ID
	}
	if id == "" || len(id) > 125 {
		return ErrInvalid
	}
	wait, added, full := peer.watchFirewallPong(id)
	if !added && !full {
		return nil
	}
	m.mu.Lock()
	if m.peers[peer.nodeID] != peer {
		m.mu.Unlock()
		peer.mu.Lock()
		if added {
			delete(peer.firewallPongs, id)
		}
		peer.mu.Unlock()
		return ErrConflict
	}
	snapshot := m.live[peer.nodeID]
	snapshot.firewallPending = id
	m.live[peer.nodeID] = snapshot
	m.mu.Unlock()
	if full {
		return ErrUnavailable
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() { peer.mu.Lock(); delete(peer.firewallPongs, id); peer.mu.Unlock() }()
		m.retryFirewallConfirmation(peer, id, wait)
	}()
	return nil
}

func (m *Master) retryFirewallConfirmation(peer *agentPeer, id string, wait <-chan struct{}) {
	// The agent's watchdog has a 45-second deadline. Continue status reads long
	// enough to resolve a lost successful command reply; never guess at rollback.
	ctx, cancel := context.WithTimeout(peer.ctx, 90*time.Second)
	defer cancel()
	attempt := 1
	for ctx.Err() == nil {
		if !m.isFirewallPending(peer, id) {
			return
		}
		ping, stopPing := context.WithTimeout(ctx, 3*time.Second)
		err := upstreamqueue.Execute(ping, peer.queue, func(context.Context) error {
			return peer.socket.WriteControl(websocket.PingMessage, []byte(id), time.Now().Add(3*time.Second))
		})
		if err == nil {
			select {
			case <-wait:
				stopPing()
				commandID := "firewall-confirm-" + id
				if attempt > 1 {
					commandID = fmt.Sprintf("%s-%d", commandID, attempt)
				}
				confirmed, failed := m.firewallConfirmAttempt(ctx, peer, id, commandID)
				if confirmed {
					return
				}
				if failed {
					attempt++
				}
			case <-ping.Done():
				stopPing()
			}
		} else {
			stopPing()
		}
		if m.firewallStatusConfirmed(ctx, peer, id) {
			return
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (m *Master) firewallConfirmAttempt(ctx context.Context, peer *agentPeer, id, commandID string) (bool, bool) {
	check, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := m.commandPeer(check, peer, commandID, "firewall.confirm", map[string]string{"transaction_id": id})
	var reply struct {
		Confirmed bool `json:"confirmed"`
	}
	var failure *CommandError
	failed := errors.As(err, &failure) && failure.Outcome == CommandFailed
	return err == nil && wire.Decode(result, &reply) == nil && reply.Confirmed && m.clearFirewallPending(peer, id), failed
}

func (m *Master) firewallStatusConfirmed(ctx context.Context, peer *agentPeer, id string) bool {
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := m.commandPeer(check, peer, "", "firewall.status", map[string]any{})
	return err == nil && confirmedFirewallStatus(result, id) && m.clearFirewallPending(peer, id)
}

func confirmedFirewallStatus(result json.RawMessage, id string) bool {
	var reply struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	return json.Unmarshal(result, &reply) == nil && reply.ID == id && reply.Status == "confirmed"
}

func (m *Master) isFirewallPending(peer *agentPeer, id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.peers[peer.nodeID] == peer && m.live[peer.nodeID].firewallPending == id
}

func (m *Master) clearFirewallPending(peer *agentPeer, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.live[peer.nodeID]
	if m.peers[peer.nodeID] != peer || current.firewallPending != id {
		return false
	}
	current.firewallPending = ""
	m.live[peer.nodeID] = current
	return true
}
