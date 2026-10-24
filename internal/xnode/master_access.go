package xnode

import (
	"context"
	"fmt"
)

func (m *Master) rotateEnrollment(ctx context.Context, id string, hash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.service.repo.RotateXNodeToken(ctx, id, hash); err != nil {
		return err
	}
	if peer := m.peers[id]; peer != nil {
		peer.cancel()
		_ = peer.socket.Close()
	}
	return nil
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
