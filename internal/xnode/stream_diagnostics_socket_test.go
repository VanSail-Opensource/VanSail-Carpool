package xnode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func TestAuthenticatedSocketDisconnectRecordsSafeTerminalCause(t *testing.T) {
	for _, test := range []struct {
		name, stage, cause string
		kind               int
		payload            []byte
	}{
		{"invalid envelope", "frame_envelope", "protocol_rejected", websocket.TextMessage, []byte(`{"private-token":`)},
		{"unnegotiated binary", "frame_kind", "protocol_rejected", websocket.BinaryMessage, []byte("private-payload")},
		{"remote policy close", "socket_read", "abnormal_close", websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "private-close")},
		{"normal close", "socket_read", "peer_closed", websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "private-close")},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			token := strings.Repeat("a", 43)
			master := NewMaster(NewService(reportSocketRepository{token: sha256.Sum256([]byte(token))}, nil, nil, nil, nil), slog.New(slog.NewJSONHandler(&output, nil)))
			master.ctx = ctx
			server := httptest.NewServer(master)
			defer server.Close()
			dialer := websocket.Dialer{Subprotocols: []string{"x-node.v1"}}
			socket, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Authorization": []string{"Bearer " + token}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = socket.Close() }()
			agent, boot := strings.Repeat("1", 32), strings.Repeat("2", 32)
			if err = socket.WriteJSON(wire.New("agent.hello", "", 0, wire.AgentHello{AgentID: agent, BootID: boot})); err != nil {
				t.Fatal(err)
			}
			if err = socket.WriteJSON(wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: agent, BootID: boot, Time: time.Now(), Ready: true})); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(2 * time.Second)
			for {
				master.mu.RLock()
				received := !master.live["node"].receivedAt.IsZero()
				master.mu.RUnlock()
				if received {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("authenticated report did not reach the live snapshot")
				}
				time.Sleep(time.Millisecond)
			}
			if err = socket.WriteMessage(test.kind, test.payload); err != nil {
				t.Fatal(err)
			}
			if err = socket.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, _, err = socket.ReadMessage(); err == nil {
				t.Fatal("invalid or closed stream remained open")
			}
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				t.Fatal("server did not close the failed stream before the deadline")
			}
			master.Wait()
			record := decodeDisconnectRecord(t, output.String())
			if record["stage"] != test.stage || record["cause"] != test.cause || record["telemetry_age_ms"].(float64) < 0 {
				t.Fatal("wrong terminal diagnostic", record)
			}
			if strings.Contains(output.String(), token) || strings.Contains(output.String(), agent) || strings.Contains(output.String(), boot) {
				t.Fatal("connection identity leaked into terminal diagnostic")
			}
		})
	}
}
