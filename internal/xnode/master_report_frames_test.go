package xnode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

type reportSocketRepository struct {
	Repository
	token [32]byte
}

func (r reportSocketRepository) XNodeNodeByToken(_ context.Context, hash []byte) (Node, error) {
	if !bytes.Equal(hash, r.token[:]) {
		return Node{}, ErrNotFound
	}
	return Node{ID: "node"}, nil
}
func (reportSocketRepository) BindXNodeAgent(context.Context, string, string) error { return nil }

func TestMixedReportFormatsNegotiateAndReplaceSnapshots(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "compact"}[modern], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			token := strings.Repeat("a", 43)
			master := NewMaster(NewService(reportSocketRepository{token: sha256.Sum256([]byte(token))}, nil, nil, nil, nil), nil)
			master.ctx = ctx
			server := httptest.NewServer(master)
			t.Cleanup(func() { cancel(); server.Close(); master.Wait() })
			headers := http.Header{"Authorization": []string{"Bearer " + token}}
			caps := []string{}
			if modern {
				headers.Set(wire.TelemetryFormatHeader, wire.CompactTelemetryFormat)
				caps = append(caps, wire.CompactTelemetryCapability)
			}
			dialer := websocket.Dialer{EnableCompression: modern, Subprotocols: []string{"x-node.v1"}}
			socket, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), headers)
			if err != nil {
				t.Fatal(err)
			}
			defer socket.Close()
			if got := response.Header.Get(wire.TelemetryFormatHeader); (got == wire.CompactTelemetryFormat) != modern {
				t.Fatal("format negotiation changed", got)
			}
			if err := socket.WriteJSON(wire.New("agent.hello", "", 0, wire.AgentHello{AgentID: strings.Repeat("1", 32), BootID: strings.Repeat("2", 32), Capabilities: caps})); err != nil {
				t.Fatal(err)
			}
			report := wire.TelemetryReport{AgentID: strings.Repeat("1", 32), BootID: strings.Repeat("2", 32), Time: time.Now().UTC(), Ready: true, Users: wire.UserMetrics{Connections: map[int]int{5: 3}}}
			send := func(value wire.TelemetryReport) {
				t.Helper()
				message := wire.New("telemetry.report", "", 0, value)
				if modern {
					message = wire.New("telemetry.compact", "", 0, wire.CompactTelemetry(value))
				}
				if err := socket.WriteJSON(message); err != nil {
					t.Fatal(err)
				}
			}
			wait := func(predicate func(liveSnapshot) bool) liveSnapshot {
				t.Helper()
				deadline := time.Now().Add(2 * time.Second)
				for {
					master.mu.RLock()
					snapshot := master.live["node"]
					master.mu.RUnlock()
					if predicate(snapshot) {
						return snapshot
					}
					if time.Now().After(deadline) {
						t.Fatal("report did not become current")
					}
					time.Sleep(time.Millisecond)
				}
			}
			send(report)
			first := wait(func(snapshot liveSnapshot) bool { return snapshot.report.Users.Connections[5] == 3 })
			report.Users = wire.UserMetrics{}
			send(report)
			wait(func(snapshot liveSnapshot) bool {
				return snapshot.receivedAt.After(first.receivedAt) && len(snapshot.report.Users.Connections) == 0
			})
		})
	}
}

func TestCompactReportsRequireNegotiationAndExplicitSnapshot(t *testing.T) {
	message := wire.New("telemetry.compact", "", 0, wire.CompactTelemetryReport{Users: []wire.CompactUserMetrics{}})
	if _, err := decodeTelemetryFrame(&agentPeer{}, message); err == nil {
		t.Fatal("unnegotiated format accepted")
	}
	if _, err := decodeTelemetryFrame(&agentPeer{compactTelemetry: true}, wire.New("telemetry.compact", "", 0, map[string]any{"a": "agent", "b": "boot"})); err == nil {
		t.Fatal("missing snapshot accepted")
	}
}

func TestMasterRejectsOversizedCompressedHelloBeforeDecoding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	token := strings.Repeat("a", 43)
	master := NewMaster(NewService(reportSocketRepository{token: sha256.Sum256([]byte(token))}, nil, nil, nil, nil), nil)
	master.ctx = ctx
	server := httptest.NewServer(master)
	defer server.Close()
	dialer := websocket.Dialer{EnableCompression: true, Subprotocols: []string{"x-node.v1"}}
	socket, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	if err := socket.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := socket.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", wire.MaxFrameBytes+1))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := socket.ReadMessage(); err == nil {
		t.Fatal("oversized expansion retained the connection")
	}
	master.mu.RLock()
	defer master.mu.RUnlock()
	if len(master.peers) != 0 {
		t.Fatal("oversized hello reached peer admission")
	}
}
