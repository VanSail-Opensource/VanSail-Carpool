package xnode

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

// Only fixed stages cross this boundary. Wrapped decoder/network errors remain
// available for classification, but their text may contain agent-controlled data.
type agentReadFailure struct {
	stage string
	cause error
}

func (e agentReadFailure) Error() string { return "XNODE_AGENT_READ_FAILED" }
func (e agentReadFailure) Unwrap() error { return e.cause }

func agentStreamCause(stage string, err error) (string, int) {
	if errors.Is(err, context.Canceled) {
		return "cancelled", 0
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout", 0
	}
	var closeError *websocket.CloseError
	if errors.As(err, &closeError) {
		if closeError.Code == websocket.CloseNormalClosure || closeError.Code == websocket.CloseGoingAway {
			return "peer_closed", closeError.Code
		}
		return "abnormal_close", closeError.Code
	}
	if errors.Is(err, net.ErrClosed) {
		return "connection_closed", 0
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "unexpected_eof", 0
	}
	if stage == "inbound_queue" {
		if errors.Is(err, ErrUnavailable) {
			return "queue_capacity", 0
		}
		return "frame_rejected", 0
	}
	if errors.Is(err, wire.ErrReportFrame) || errors.Is(err, ErrInvalid) || stage == "frame_envelope" || stage == "frame_kind" || stage == "report_format" {
		return "protocol_rejected", 0
	}
	if stage == "frame_payload" {
		return "payload_failure", 0
	}
	return "transport_failure", 0
}

// Exactly one terminal event is emitted by the authenticated reader before it
// closes. Never log frame types, payloads, close text, URLs or raw error strings.
func (m *Master) recordAgentDisconnect(peer *agentPeer, stage, lane string, err error) {
	if m.logger == nil {
		return
	}
	var failure agentReadFailure
	if errors.As(err, &failure) {
		stage = failure.stage
	}
	cause, closeCode := agentStreamCause(stage, err)
	level := slog.LevelWarn
	if cause == "peer_closed" || cause == "cancelled" || peer.ctx != nil && peer.ctx.Err() != nil {
		level = slog.LevelInfo
	}
	m.mu.RLock()
	receivedAt := m.live[peer.nodeID].receivedAt
	current := m.peers[peer.nodeID] == peer
	m.mu.RUnlock()
	age := int64(-1)
	if current && !receivedAt.IsZero() {
		age = max(0, m.service.now().Sub(receivedAt).Milliseconds())
	}
	fields := []any{"node_id", peer.nodeID, "error_code", "XNODE_AGENT_DISCONNECTED", "stage", stage, "cause", cause, "current_peer", current, "telemetry_age_ms", age}
	if lane != "" {
		fields = append(fields, "lane", lane)
	}
	if closeCode != 0 {
		fields = append(fields, "close_code", closeCode)
	}
	m.logger.Log(context.Background(), level, "agent connection ended", fields...)
}

// Frame type is untrusted. Reduce it to the fixed receiving-lane vocabulary.
func inboundLaneName(kind string) string {
	if strings.HasPrefix(kind, "hourly.") {
		return "hourly"
	}
	switch kind {
	case "command.accepted", "command.result", "command.progress":
		return "results"
	case "error":
		// Correlation may retire concurrently after offer selected its lane.
		return "control_or_results"
	case "compute.claim", "compute.heartbeat", "compute.result", "compute.cancel":
		return "compute"
	case "telemetry.report", "telemetry.compact":
		return "telemetry"
	case "traffic.report":
		return "traffic"
	case "logs.report":
		return "logs"
	}
	return "control"
}
