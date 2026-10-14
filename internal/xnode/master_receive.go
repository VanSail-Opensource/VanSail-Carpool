package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) receive(ctx context.Context, peer *agentPeer, message wire.Message) error {
	switch message.Type {
	case "state.request":
		return m.syncNode(ctx, peer, true)
	case "state.accepted", "state.applied":
		peer.mu.Lock()
		if message.Revision > peer.revision {
			peer.mu.Unlock()
			return ErrInvalid
		}
		peer.mu.Unlock()
		return m.service.repo.AcknowledgeXNodeState(ctx, peer.nodeID, message.Revision, message.Type == "state.applied")
	case "state.failed", "error":
		peer.mu.Lock()
		peer.digest = ""
		peer.mu.Unlock()
		return nil
	case "telemetry.report":
		var report wire.TelemetryReport
		if err := wire.Decode(message.Data, &report); err != nil {
			return ErrInvalid
		}
		if report.AgentID != peer.agentID || report.BootID != peer.bootID || !validMetrics(report.Users) {
			return ErrInvalid
		}
		m.mu.Lock()
		snapshot := m.live[peer.nodeID]
		snapshot.report = report
		snapshot.receivedAt = m.service.now()
		if report.Ready && report.Synchronized {
			snapshot.hello.UpgradePending = false
		}
		m.live[peer.nodeID] = snapshot
		m.mu.Unlock()
		return nil
	case "traffic.report":
		var report wire.TrafficReport
		if err := wire.Decode(message.Data, &report); err != nil {
			return ErrInvalid
		}
		raw, ok := strings.CutPrefix(report.ID, peer.agentID+":")
		if !ok || message.ID != report.ID {
			return ErrInvalid
		}
		sequence, err := strconv.ParseUint(raw, 10, 63)
		if err != nil {
			return ErrInvalid
		}
		normalized, err := json.Marshal(report)
		if err != nil {
			return ErrInvalid
		}
		digest := sha256.Sum256(normalized)
		if err = m.service.repo.RecordXNodeTraffic(ctx, peer.nodeID, peer.agentID, sequence, hex.EncodeToString(digest[:]), report); err != nil {
			return err
		}
		return peer.send(ctx, wire.New("traffic.ack", "", 0, map[string]string{"id": report.ID}))
	case "command.result", "command.cancelled":
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
		snapshot := m.live[peer.nodeID]
		snapshot.logs = append(snapshot.logs, report.Entries...)
		if len(snapshot.logs) > 500 {
			snapshot.logs = snapshot.logs[len(snapshot.logs)-500:]
		}
		m.live[peer.nodeID] = snapshot
		m.mu.Unlock()
		return nil
	case "firewall.pending":
		return m.confirmFirewall(ctx, peer, message)
	case "command.accepted", "command.progress", "admission.rejected", "torrent.detected", "torrent.expired", "agent.error":
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
