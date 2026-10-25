package xnode

import (
	"context"
	"encoding/json"
	"math"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) receive(ctx context.Context, peer *agentPeer, message wire.Message) error {
	switch message.Type {
	case "state.request":
		m.requestSync(peer)
		return nil
	case "state.accepted", "state.applied":
		peer.mu.Lock()
		if message.Revision > peer.revision {
			peer.mu.Unlock()
			return ErrInvalid
		}
		peer.mu.Unlock()
		if err := m.service.repo.AcknowledgeXNodeState(ctx, peer.nodeID, message.Revision, message.Type == "state.applied"); err != nil {
			return err
		}
		if message.Type == "state.applied" {
			peer.mu.Lock()
			if message.Revision == peer.revision {
				if peer.digest == "" {
					peer.digest = peer.failedDigest
				}
				peer.failedDigest, peer.retryDelay, peer.retryAt = "", 0, time.Time{}
			}
			peer.mu.Unlock()
		}
		return nil
	case "state.failed", "error":
		peer.mu.Lock()
		commandError := false
		if message.Type == "error" {
			if pending := peer.pending[message.ID]; pending != nil {
				commandError = true
				select {
				case pending <- message:
				default:
				}
			}
		}
		// Command failures and superseded activation results do not invalidate
		// the current configuration. Retry an actual rejection with bounded pace.
		if !commandError && message.Revision > 0 && message.Revision == peer.revision {
			if peer.digest != "" {
				peer.failedDigest = peer.digest
			}
			peer.digest = ""
			peer.retryDelay = min(time.Minute, max(time.Second, peer.retryDelay*2))
			peer.retryAt = m.service.now().Add(peer.retryDelay)
		}
		peer.mu.Unlock()
		return nil
	case "telemetry.report", "telemetry.compact":
		report, err := decodeTelemetryFrame(peer, message)
		if err != nil {
			return ErrInvalid
		}
		if report.AgentID != peer.agentID || report.BootID != peer.bootID || !validMetrics(report.Users) {
			return ErrInvalid
		}
		m.mu.Lock()
		if m.peers[peer.nodeID] != peer {
			m.mu.Unlock()
			return ErrConflict
		}
		snapshot := m.live[peer.nodeID]
		snapshot.report = report
		snapshot.receivedAt = m.service.now()
		if report.Ready && report.Synchronized {
			snapshot.hello.UpgradePending = false
		}
		m.live[peer.nodeID] = snapshot
		m.mu.Unlock()
		m.rememberNodeIPs(peer, report.Machine)
		return nil
	case "traffic.report":
		return m.receiveTraffic(ctx, peer, message)
	case "command.accepted", "command.result":
		peer.mu.Lock()
		pending := peer.pending[message.ID]
		peer.mu.Unlock()
		if pending != nil {
			select {
			case pending <- message:
			default:
			}
		}
		return nil
	case "logs.report":
		var report struct {
			Entries []json.RawMessage `json:"entries"`
			Dropped uint64            `json:"dropped"`
		}
		if err := wire.Decode(message.Data, &report); err != nil {
			return ErrInvalid
		}
		if len(report.Entries) > 500 || len(message.Data) > 256<<10 {
			return ErrInvalid
		}
		m.mu.Lock()
		if m.peers[peer.nodeID] != peer {
			m.mu.Unlock()
			return ErrConflict
		}
		snapshot := m.live[peer.nodeID]
		snapshot.logs = append(snapshot.logs, report.Entries...)
		snapshot.logsDropped += min(report.Dropped, ^uint64(0)-snapshot.logsDropped)
		if len(snapshot.logs) > 500 {
			snapshot.logs = snapshot.logs[len(snapshot.logs)-500:]
		}
		m.live[peer.nodeID] = snapshot
		m.mu.Unlock()
		return nil
	case "firewall.pending":
		return m.confirmFirewall(ctx, peer, message)
	case "firewall.events":
		return m.receiveFirewallEvents(ctx, peer, message)
	case "command.cancelled", "command.progress", "admission.rejected", "torrent.detected", "torrent.expired", "agent.error":
		return nil
	default:
		return ErrInvalid
	}
}

func validMetrics(metrics wire.UserMetrics) bool {
	var total int64
	for id, count := range metrics.Connections {
		if id <= 0 || count < 0 || int64(count) > math.MaxInt64-total {
			return false
		}
		total += int64(count)
	}
	for id, rate := range metrics.Rates {
		if id <= 0 || rate.SampleSeconds <= 0 || rate.UploadBytesPerSecond < 0 || rate.DownloadBytesPerSecond < 0 || math.IsNaN(rate.UploadBytesPerSecond) || math.IsNaN(rate.DownloadBytesPerSecond) || math.IsInf(rate.UploadBytesPerSecond, 0) || math.IsInf(rate.DownloadBytesPerSecond, 0) {
			return false
		}
	}
	for id, status := range metrics.Status {
		if id <= 0 || status.UserID != id {
			return false
		}
	}
	return true
}
