package xnode

import (
	"context"
	"fmt"
	"time"
)

func (m *Master) rotateEnrollment(ctx context.Context, id string, hash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.service.repo.RotateXNodeToken(ctx, id, hash); err != nil {
		return err
	}
	m.disconnectPeerLocked(id)
	return nil
}

// Invalidate identity before closing the transport: queued reports must lose
// authority immediately, even while their receiver goroutines are stopping.
func (m *Master) disconnectPeerLocked(id string) {
	peer := m.peers[id]
	delete(m.peers, id)
	snapshot := m.live[id]
	snapshot.receivedAt = time.Time{}
	m.live[id] = snapshot
	if peer != nil {
		if peer.cancel != nil {
			peer.cancel()
		}
		if peer.socket != nil {
			_ = peer.socket.Close()
		}
	}
}

func (m *Master) deleteNode(ctx context.Context, id string) error {
	// Share registration's lock so a handshake cannot join between the drain
	// check and the durable deletion. A later handshake rechecks its token.
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.peers[id] != nil {
		return fmt.Errorf("%w: enrolled node must be drained before deletion", ErrConflict)
	}
	return m.service.repo.DeleteXNodeNode(ctx, id)
}

func (m *Master) Connected(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.peers[id] != nil
}

// Version returns the release reported by the connected agent handshake.
func (m *Master) Version(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.live[id].hello.Version
}

// Sync publishes the current target or returns an error while a prerequisite is
// pending. Runtime activation remains a separate WaitApplied barrier.
func (m *Master) Sync(ctx context.Context, id string) error {
	m.mu.RLock()
	peer := m.peers[id]
	m.mu.RUnlock()
	if peer == nil {
		return ErrUnavailable
	}
	return m.syncNode(ctx, peer, false)
}

type deploymentMaster interface {
	Sync(context.Context, string) error
	WaitApplied(context.Context, string) error
}

// syncDeploymentNodes publishes the whole phase before waiting for any node.
// Completion still requires every node's applied-state barrier.
func syncDeploymentNodes(ctx context.Context, master deploymentMaster, nodes []string) error {
	for _, id := range nodes {
		if err := master.Sync(ctx, id); err != nil {
			return fmt.Errorf("deployment sync node %s: %w", id, err)
		}
	}
	for _, id := range nodes {
		if err := master.WaitApplied(ctx, id); err != nil {
			return fmt.Errorf("deployment apply node %s: %w", id, err)
		}
	}
	return nil
}
