package xnode

import (
	"context"
	"encoding/json"
	"slices"
	"time"
)

// NodeMachineMetrics is an internal, memory-only health projection. Machine
// retains the agent's exact JSON numbers; browser conversion belongs at HTTP.
type NodeMachineMetrics struct {
	NodeID                    string
	Enabled, Connected, Ready bool
	Synchronized, Stale       bool
	ReceivedAt, ValidUntil    time.Time
	ReportedAt                *time.Time
	Machine                   json.RawMessage
}

type nodeLiveObservation struct {
	snapshot liveSnapshot
	peer     *agentPeer
}

// captureNodeObservations copies immutable report banks under the fleet lock.
// User aggregation and machine decoding must not delay incoming snapshots.
func (m *Master) captureNodeObservations(nodes []Node) map[string]nodeLiveObservation {
	observations := make(map[string]nodeLiveObservation, len(nodes))
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, node := range nodes {
		observations[node.ID] = nodeLiveObservation{m.live[node.ID], m.peers[node.ID]}
	}
	return observations
}

// MachineMetrics avoids member ownership queries, activity aggregation and
// browser serialization when an internal consumer only needs machine health.
// Freshness remains tied to authenticated receipt, never to the polling time.
func (m *Master) MachineMetrics(ctx context.Context) ([]NodeMachineMetrics, error) {
	nodes, err := m.service.repo.XNodeNodes(ctx)
	if err != nil {
		return nil, err
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return nil, err
	}
	now := m.service.now()
	observations := m.captureNodeObservations(nodes)
	result := make([]NodeMachineMetrics, 0, len(nodes))
	for _, node := range nodes {
		live := observations[node.ID]
		snapshot := live.snapshot
		fresh := node.Enabled && live.peer != nil && freshSnapshot(snapshot, now, global.Telemetry.ReportSeconds)
		item := NodeMachineMetrics{
			NodeID: node.ID, Enabled: node.Enabled, Connected: live.peer != nil,
			Ready: snapshot.report.Ready, Synchronized: snapshot.report.Synchronized && snapshot.firewallPending == "",
			Stale: !fresh, ReceivedAt: snapshot.receivedAt, Machine: json.RawMessage(`{}`),
		}
		if !snapshot.receivedAt.IsZero() {
			item.ValidUntil = snapshot.receivedAt.Add(reportWindow(global.Telemetry.ReportSeconds))
		}
		if fresh {
			at := snapshot.report.Time
			item.ReportedAt = &at
			if len(snapshot.report.Machine) != 0 {
				item.Machine = slices.Clone(snapshot.report.Machine)
			}
		}
		result = append(result, item)
	}
	return result, nil
}
