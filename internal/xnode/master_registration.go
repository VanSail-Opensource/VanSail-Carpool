package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Enrollment rotation and peer replacement share the node's admission lock.
// Stream replacement precedes publication without holding the fleet-wide mutex.
func (m *Master) registerPeer(ctx context.Context, node string, hash []byte, hello wire.AgentHello, peer *agentPeer) error {
	release, err := m.lockEnrollment(ctx, node)
	if err != nil {
		return err
	}
	defer release()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() != nil || m.ctx != nil && m.ctx.Err() != nil {
		return ErrUnavailable
	}
	if previous := m.peers[node]; previous != nil {
		previous.cancel()
		previous.socket.Close()
	}
	m.peers[node] = peer
	previous := m.live[node]
	snapshot := liveSnapshot{hello: hello, activityReports: peer.activityReports, ddnsWarningAt: previous.ddnsWarningAt, ddnsWarning: previous.ddnsWarning}
	// The current master-side utility diagnostic belongs to stored node settings,
	// not the previous agent's authority or telemetry. Preserve only that entry.
	if snapshot.ddnsWarning != nil {
		snapshot.logs = []json.RawMessage{snapshot.ddnsWarning}
	}
	m.live[node] = snapshot
	m.wg.Add(1)
	return nil
}
