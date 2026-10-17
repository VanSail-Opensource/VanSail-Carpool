package xnode

import (
	"context"
	"encoding/json"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) Metrics(ctx context.Context) (MetricsSummary, error) {
	nodes, err := m.service.repo.XNodeNodes(ctx)
	if err != nil {
		return MetricsSummary{}, err
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return MetricsSummary{}, err
	}
	now := m.service.now()
	identities, err := m.service.repo.XNodeIdentities(ctx)
	if err != nil {
		return MetricsSummary{}, err
	}
	paid := map[int64]bool{}
	for _, identity := range identities {
		if identity.UserID != "" {
			paid[identity.ProxyUserID] = true
		}
	}
	result := MetricsSummary{Nodes: []NodeMetrics{}, TotalNodes: len(nodes), FetchedAt: now}
	m.mu.RLock()
	for _, node := range nodes {
		snapshot := m.live[node.ID]
		_, connected := m.peers[node.ID]
		fresh := connected && freshSnapshot(snapshot, now, global.Telemetry.ReportSeconds)
		item := NodeMetrics{NodeID: node.ID, Connected: connected, Ready: snapshot.report.Ready, Synchronized: snapshot.report.Synchronized && snapshot.firewallPending == "", Stale: !fresh, Version: snapshot.hello.Version, XrayVersion: snapshot.hello.XrayVersion, Capabilities: append([]string{}, snapshot.hello.Capabilities...), Machine: json.RawMessage(`{}`), Certificates: json.RawMessage(`{}`), Logs: append([]json.RawMessage{}, snapshot.logs...)}
		item.Firewall = snapshot.firewall
		if !snapshot.receivedAt.IsZero() {
			at := snapshot.report.Time
			item.ReportedAt = &at
			item.Machine = snapshot.report.Machine
			item.Certificates = snapshot.report.Certificates
			if len(item.Machine) == 0 {
				item.Machine = json.RawMessage(`{}`)
			}
			if len(item.Certificates) == 0 {
				item.Certificates = json.RawMessage(`{}`)
			}
		}
		if fresh {
			result.OnlineNodes++
			for id, count := range snapshot.report.Users.Connections {
				item.ActiveConnections += int64(count)
				if count > 0 && paid[int64(id)] {
					item.UsersOnline++
				}
			}
			for _, rate := range snapshot.report.Users.Rates {
				item.UploadBytesPerSecond += rate.UploadBytesPerSecond
				item.DownloadBytesPerSecond += rate.DownloadBytesPerSecond
			}
			result.ActiveConnections += item.ActiveConnections
			result.UploadBytesPerSecond += item.UploadBytesPerSecond
			result.DownloadBytesPerSecond += item.DownloadBytesPerSecond
		}
		result.Nodes = append(result.Nodes, item)
	}
	m.mu.RUnlock()
	for index := range result.Nodes {
		result.Nodes[index].Machine = BrowserMetricNumbers(result.Nodes[index].Machine)
	}
	return result, nil
}

func freshSnapshot(snapshot liveSnapshot, now time.Time, reportSeconds int) bool {
	if reportSeconds <= 0 {
		reportSeconds = 5
	}
	maximum := time.Duration(reportSeconds*3) * time.Second
	if maximum < 15*time.Second {
		maximum = 15 * time.Second
	}
	return !snapshot.receivedAt.IsZero() && now.Sub(snapshot.receivedAt) <= maximum
}

func (m *Master) Blocked(id int) bool { m.mu.RLock(); defer m.mu.RUnlock(); return m.globalBlocks[id] }
func (m *Master) Capabilities(nodeID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.live[nodeID].hello.Capabilities...)
}

func (m *Master) nodeReport(nodeID string) wire.TelemetryReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return wire.Clone(m.live[nodeID].report)
}
