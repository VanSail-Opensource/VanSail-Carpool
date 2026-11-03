package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Enrollment rotation and peer replacement share the master lock. Persist stream
// replacement in the same critical section so concurrent hellos cannot publish a
// peer whose boot has already been durably closed by another authenticated peer.
func (m *Master) registerPeer(ctx context.Context, node string, hash []byte, hello wire.AgentHello, peer *agentPeer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	current, err := m.service.repo.XNodeNodeByToken(ctx, hash)
	if err != nil || current.ID != node {
		return ErrUnavailable
	}
	if err = m.service.repo.BindXNodeAgent(ctx, node, hello.AgentID); err != nil {
		return err
	}
	if streams, ok := m.service.repo.(TrafficStreamRepository); ok {
		stream := ""
		if peer.trafficStreams {
			stream = hello.BootID
		}
		if err = streams.RegisterXNodeTrafficStream(ctx, node, hello.AgentID, stream); err != nil {
			return err
		}
	}
	if previous := m.peers[node]; previous != nil {
		previous.cancel()
		previous.socket.Close()
	}
	m.peers[node] = peer
	m.live[node] = liveSnapshot{hello: hello}
	m.wg.Add(1)
	return nil
}
