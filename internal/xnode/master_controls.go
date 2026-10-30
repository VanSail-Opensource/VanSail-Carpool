package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) syncControls(ctx context.Context, peer *agentPeer, current, previous map[int]bool) error {
	users := []wire.UserControl{}
	for id, blocked := range current {
		if old, ok := previous[id]; !ok || old != blocked {
			users = append(users, wire.UserControl{ID: id, BlockNew: blocked})
		}
	}
	if len(users) == 0 {
		return nil
	}
	peer.mu.Lock()
	base := peer.revision
	peer.mu.Unlock()
	revision, err := m.service.repo.AdvanceXNodeRevision(ctx, peer.nodeID, base)
	if err != nil {
		return err
	}
	message := wire.New("user.controls", "", revision, wire.UserControls{Users: users})
	message.BaseRevision = base
	peer.mu.Lock()
	peer.revision = revision
	peer.lastControls = current
	peer.mu.Unlock()
	if err = peer.send(ctx, message); err != nil {
		peer.mu.Lock()
		peer.digest = ""
		peer.mu.Unlock()
		return err
	}
	return nil
}
