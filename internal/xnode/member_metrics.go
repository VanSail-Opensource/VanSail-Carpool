package xnode

import "context"

func (m *Master) UserMetrics(ctx context.Context, userID string) (UserMetrics, error) {
	identity, err := m.service.repo.XNodeIdentityByUser(ctx, userID)
	if err != nil {
		return UserMetrics{}, err
	}
	nodes, reasons, err := m.service.metricNodeScope(ctx, identity)
	if err != nil {
		return UserMetrics{}, err
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return UserMetrics{}, err
	}
	result := UserMetrics{Reasons: reasons, Limited: len(reasons) > 0, FetchedAt: m.service.now()}
	id := int(identity.ProxyUserID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for nodeID := range nodes {
		snapshot := m.live[nodeID]
		if !nodes[nodeID] || m.peers[nodeID] == nil || !activityAvailable(snapshot, result.FetchedAt, global.Telemetry.ReportSeconds) {
			result.Incomplete = true
			continue
		}
		result.Connections += int64(snapshot.report.Users.Connections[id])
		rate := snapshot.report.Users.Rates[id]
		result.UploadBytesPerSecond += rate.UploadBytesPerSecond
		result.DownloadBytesPerSecond += rate.DownloadBytesPerSecond
		status := snapshot.report.Users.Status[id]
		if status.Limited {
			reason := status.Reason
			if reason == "block_new" && m.globalBlocks[id] {
				reason = "global_connection_limit"
			}
			result.Reasons = append(result.Reasons, UserLimit{NodeID: nodeID, Reason: reason, Scope: status.Scope})
			result.Limited = true
		}
	}
	if m.globalBlocks[id] {
		result.Limited = true
		if len(result.Reasons) == 0 {
			result.Reasons = append(result.Reasons, UserLimit{Reason: "global_connection_limit", Scope: "new_connection"})
		}
	}
	return result, nil
}
