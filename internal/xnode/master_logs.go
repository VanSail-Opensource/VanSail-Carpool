package xnode

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// NodeLogs projects the bounded, memory-only logs.report buffer. Reads never
// issue agent commands or store source addresses/log text in the database.
type NodeLogs struct {
	NodeID    string            `json:"nodeId"`
	Connected bool              `json:"connected"`
	Stale     bool              `json:"stale"`
	Entries   []json.RawMessage `json:"entries"`
	Dropped   string            `json:"dropped"`
	FetchedAt time.Time         `json:"fetchedAt"`
}

func (m *Master) Logs(ctx context.Context, nodeID string) (NodeLogs, error) {
	if _, err := m.service.repo.XNodeNode(ctx, nodeID); err != nil {
		return NodeLogs{}, err
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return NodeLogs{}, err
	}
	now := m.service.now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := m.live[nodeID]
	_, connected := m.peers[nodeID]
	return NodeLogs{NodeID: nodeID, Connected: connected, Stale: !connected || !freshSnapshot(snapshot, now, global.Telemetry.ReportSeconds), Entries: append([]json.RawMessage{}, snapshot.logs...), Dropped: strconv.FormatUint(snapshot.logsDropped, 10), FetchedAt: now}, nil
}
