package xnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func TestAgentDisconnectDiagnosticsClassifyWithoutRawErrors(t *testing.T) {
	for _, test := range []struct {
		name, stage, cause string
		err                error
		closeCode          int
	}{
		{"context deadline", "socket_read", "timeout", context.DeadlineExceeded, 0},
		{"network timeout", "frame_payload", "timeout", &net.DNSError{Name: "private-host", Err: "private-detail", IsTimeout: true}, 0},
		{"cancelled", "socket_read", "cancelled", context.Canceled, 0},
		{"normal close", "socket_read", "peer_closed", &websocket.CloseError{Code: 1000, Text: "private-close"}, 1000},
		{"abnormal close", "socket_read", "abnormal_close", &websocket.CloseError{Code: 1008, Text: "private-close"}, 1008},
		{"closed socket", "read_deadline", "connection_closed", net.ErrClosed, 0},
		{"truncated payload", "frame_payload", "unexpected_eof", io.ErrUnexpectedEOF, 0},
		{"compressed frame", "frame_payload", "protocol_rejected", wire.ErrReportFrame, 0},
		{"invalid envelope", "frame_envelope", "protocol_rejected", errors.New("private-json"), 0},
		{"wrong frame kind", "frame_kind", "protocol_rejected", ErrInvalid, 0},
		{"negotiated format", "report_format", "protocol_rejected", ErrInvalid, 0},
		{"payload bounds", "frame_payload", "payload_failure", errors.New("private-payload"), 0},
		{"transport failure", "socket_read", "transport_failure", errors.New("private-network"), 0},
		{"queue capacity", "inbound_queue", "queue_capacity", ErrUnavailable, 0},
		{"invalid queued frame", "inbound_queue", "frame_rejected", ErrInvalid, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			master := NewMaster(NewService(nil, nil, nil, nil, nil), slog.New(slog.NewJSONHandler(&output, nil)))
			now := time.Now().UTC()
			master.service.now = func() time.Time { return now }
			peer := &agentPeer{nodeID: "node", ctx: context.Background()}
			master.peers[peer.nodeID] = peer
			master.live[peer.nodeID] = liveSnapshot{receivedAt: now.Add(-5 * time.Second)}
			err := agentReadFailure{test.stage, fmt.Errorf("private wrapper: %w", test.err)}
			if !errors.Is(err, test.err) {
				t.Fatal("diagnostic wrapper lost original error identity")
			}
			master.recordAgentDisconnect(peer, "socket_read", "", err)
			record := decodeDisconnectRecord(t, output.String())
			if record["cause"] != test.cause || record["stage"] != test.stage || record["current_peer"] != true || record["telemetry_age_ms"] != float64(5000) {
				t.Fatal("incorrect disconnect evidence", record)
			}
			if test.closeCode != 0 && record["close_code"] != float64(test.closeCode) {
				t.Fatal("lost numeric close code", record)
			}
			output.Reset()
			master.peers[peer.nodeID] = &agentPeer{nodeID: peer.nodeID}
			master.recordAgentDisconnect(peer, "socket_read", "", err)
			if record = decodeDisconnectRecord(t, output.String()); record["telemetry_age_ms"] != float64(-1) || record["current_peer"] != false {
				t.Fatal("replacement telemetry attributed to old connection", record)
			}
		})
	}
}

func TestInboundQueueDisconnectDiagnosticsBoundLaneAndPreserveCapacity(t *testing.T) {
	for _, test := range []struct{ kind, lane string }{
		{"state.accepted", "control"}, {"private-type", "control"},
		{"command.result", "results"}, {"compute.result", "compute"}, {"traffic.report", "traffic"},
	} {
		t.Run(test.lane+test.kind, func(t *testing.T) {
			var output bytes.Buffer
			master := NewMaster(NewService(nil, nil, nil, nil, nil), slog.New(slog.NewJSONHandler(&output, nil)))
			peer := &agentPeer{nodeID: "node", ctx: context.Background()}
			master.peers[peer.nodeID] = peer
			r := &peerReceivers{master: master, peer: peer, control: newInboundLane(1), results: newInboundLane(1), compute: newInboundLane(1), traffic: newInboundLane(1)}
			message := wire.New(test.kind, "private-correlation", 0, map[string]string{"secret": "private-payload"})
			if err := r.offer(message); err != nil {
				t.Fatal(err)
			}
			err := r.offer(message)
			if !errors.Is(err, ErrUnavailable) {
				t.Fatal("full lane did not reject durable frame", err)
			}
			master.recordAgentDisconnect(peer, "inbound_queue", inboundLaneName(message.Type), err)
			record := decodeDisconnectRecord(t, output.String())
			if record["lane"] != test.lane || record["cause"] != "queue_capacity" || record["telemetry_age_ms"] != float64(-1) {
				t.Fatal("queue rejection misclassified", record)
			}
			if test.lane == "traffic" && master.trafficInboundBytes.Load() != int64(len(message.Data)+128) {
				t.Fatal("diagnostics changed retained traffic byte accounting")
			}
		})
	}
}

func decodeDisconnectRecord(t *testing.T, output string) map[string]any {
	t.Helper()
	if strings.Contains(output, "private") || strings.Count(strings.TrimSpace(output), "\n") != 0 {
		t.Fatal("disconnect diagnostics leaked private text or emitted more than one event", output)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(output), &record); err != nil {
		t.Fatal(err)
	}
	if record["error_code"] != "XNODE_AGENT_DISCONNECTED" || record["node_id"] != "node" || record["error"] != nil || record["type"] != nil {
		t.Fatal("unsafe or uncorrelated diagnostic", record)
	}
	return record
}
