package xnode_test

import (
	"context"
	"fmt"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const fixtureAgent = "0123456789abcdef0123456789abcdef"
const fixtureBoot = "abcdef0123456789abcdef0123456789"

func agentSocket(t *testing.T, server *httptest.Server, token, agent string, revision uint64) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{Subprotocols: []string{"x-node.v1"}}
	socket, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("agent connection=%v,response=%v", err, response)
	}
	t.Cleanup(func() { _ = socket.Close() })
	_ = socket.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err = socket.WriteJSON(wire.New("agent.hello", "", revision, wire.AgentHello{AgentID: agent, BootID: fixtureBoot, Version: "fixture", XrayVersion: "26.10.10", ConfigRevision: revision, Capabilities: []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "config_validation_v1", "measured_user_rates", "sampled_activity_v1", "authorization_expiry", "member_outbound_guards"}})); err != nil {
		t.Fatal(err)
	}
	return socket
}
func readFrame(t *testing.T, socket *websocket.Conn, kind string) wire.Message {
	t.Helper()
	for {
		var message wire.Message
		if err := socket.ReadJSON(&message); err != nil {
			t.Fatalf("read %s: %v", kind, err)
		}
		if message.Type == kind {
			return message
		}
	}
}
func TestEnrollmentReconnectAndReplayUseAuthoritativeRevisions(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	dialer := websocket.Dialer{Subprotocols: []string{"x-node.v1"}}
	_, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Authorization": []string{"Bearer " + strings.Repeat("a", 43)}})
	if err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatal("unenrolled token accepted")
	}
	socket := agentSocket(t, server, token, fixtureAgent, 17)
	if err = socket.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	baseline := readFrame(t, socket, "state.sync")
	if baseline.Revision <= 17 {
		t.Fatal("revision did not advance past recovered agent state")
	}
	if err = socket.WriteJSON(wire.New("state.accepted", "", baseline.Revision, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	report := wire.TrafficReport{ID: fixtureAgent + ":1", Traffic: wire.Traffic{}}
	for i := 0; i < 2; i++ {
		if err = socket.WriteJSON(wire.New("traffic.report", report.ID, 0, report)); err != nil {
			t.Fatal(err)
		}
		readFrame(t, socket, "traffic.ack")
	}
	sequence, err := f.store.XNodeReportSequence(ctx, fixtureAgent)
	if err != nil || sequence != 1 {
		t.Fatalf("replayed cursor=%d,error=%v", sequence, err)
	}
	_ = socket.Close()
	next := agentSocket(t, server, token, fixtureAgent, baseline.Revision)
	if err = next.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	snapshot := readFrame(t, next, "state.sync")
	if snapshot.Revision <= baseline.Revision {
		t.Fatal("reconnect reused an old revision")
	}
	var config wire.Config
	if err = wire.Decode(snapshot.Data, &config); err != nil || config.Users == nil || len(config.Users) != 0 {
		t.Fatal("reconnect omitted authoritative empty authorization")
	}
	_, newToken, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil || newToken == token {
		t.Fatal("enrollment token did not rotate")
	}
	if _, _, err = dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Authorization": []string{"Bearer " + token}}); err == nil {
		t.Fatal("revoked enrollment token accepted")
	}
}
func TestNodeSnapshotsReplaceCountsWithoutPersistingTelemetry(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0)
	if err = socket.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	readFrame(t, socket, "state.sync")
	for _, count := range []int{3, 0} {
		connections := map[int]int{}
		if count > 0 {
			connections[int(f.identity.ProxyUserID)] = count
		}
		report := wire.TelemetryReport{AgentID: fixtureAgent, BootID: fixtureBoot, Time: time.Now(), Ready: true, Synchronized: true, Users: wire.UserMetrics{Connections: connections}}
		if err = socket.WriteJSON(wire.New("telemetry.report", "", 0, report)); err != nil {
			t.Fatal(err)
		}
		// A following durable ACK proves the serial reader processed the snapshot.
		sequence := uint64(1)
		if count == 0 {
			sequence = 2
		}
		batch := wire.TrafficReport{ID: fmt.Sprintf("%s:%d", fixtureAgent, sequence), Traffic: wire.Traffic{}}
		if err = socket.WriteJSON(wire.New("traffic.report", batch.ID, 0, batch)); err != nil {
			t.Fatal(err)
		}
		readFrame(t, socket, "traffic.ack")
		metrics, err := f.master.Metrics(ctx)
		if err != nil || metrics.ActiveConnections != int64(count) {
			t.Fatalf("authoritative count=%d,error=%v", metrics.ActiveConnections, err)
		}
	}
	rows, err := f.store.DB().QueryContext(ctx, `SELECT name,sql FROM sqlite_master WHERE type='table' AND name LIKE 'xnode_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, ddl string
		if err = rows.Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"connections", "active_ips", "rejections", "bytes_per_second"} {
			if strings.Contains(ddl, field) {
				t.Fatalf("live telemetry entered %s", name)
			}
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
}
