package xnode

import "context"

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
func (m *Master) Sync(ctx context.Context, id string) error {
	m.mu.RLock()
	peer := m.peers[id]
	m.mu.RUnlock()
	if peer == nil {
		return ErrUnavailable
	}
	return m.syncNode(ctx, peer, false)
}
