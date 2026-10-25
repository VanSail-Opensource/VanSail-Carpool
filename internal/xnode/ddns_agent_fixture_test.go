package xnode_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

type ddnsFixtureAgent struct {
	socket      *websocket.Conn
	writeMu, mu sync.Mutex
	jobs        []wire.DDNSJob
	protected   []string
	pending     []wire.Message
	hold        bool
	acks        chan string
	done        chan struct{}
}

func newDDNSFixtureAgent(t *testing.T, ctx context.Context, f fixture, server *httptest.Server, node, agent string) *ddnsFixtureAgent {
	t.Helper()
	_, token, err := f.service.Enrollment(ctx, node)
	if err != nil {
		t.Fatal(err)
	}
	headers := http.Header{"Authorization": []string{"Bearer " + token}, wire.DDNSChangesHeader: []string{wire.DDNSChangesFormat}}
	dialer := websocket.Dialer{Subprotocols: []string{"x-node.v1"}}
	socket, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), headers)
	if err != nil {
		t.Fatal(err)
	}
	if response.Header.Get(wire.DDNSChangesHeader) != wire.DDNSChangesFormat {
		t.Fatal("DDNS negotiation missing")
	}
	a := &ddnsFixtureAgent{socket: socket, acks: make(chan string, 16), done: make(chan struct{})}
	capabilities := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "sampled_activity_v1", "config_validation_v1", "transit_credentials_v1", "authorization_expiry", "member_outbound_guards", "firewall_control_v1", "firewall_master_observation_v1", wire.BuiltinDDNSCapability, wire.DDNSChangesCapability}
	if err = a.send(wire.New("agent.hello", "", 0, wire.AgentHello{AgentID: agent, BootID: fixtureBoot, Version: "fixture", XrayVersion: "26.10.17", Capabilities: capabilities})); err != nil {
		t.Fatal(err)
	}
	go func() { defer close(a.done); a.run(ctx, agent) }()
	t.Cleanup(func() { _ = socket.Close(); <-a.done })
	return a
}

func (a *ddnsFixtureAgent) send(message wire.Message) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.socket.WriteJSON(message)
}
func (a *ddnsFixtureAgent) result(message wire.Message, name string, data any) error {
	return a.send(wire.New("command.result", message.ID, 0, map[string]any{"id": message.ID, "command": name, "status": "completed", "data": data, "finished": time.Now()}))
}
func (a *ddnsFixtureAgent) releaseProtected() error {
	a.mu.Lock()
	a.hold = false
	pending := a.pending
	a.pending = nil
	a.mu.Unlock()
	for _, message := range pending {
		if err := a.result(message, "firewall.protected.sync", map[string]any{}); err != nil {
			return err
		}
	}
	return nil
}
func (a *ddnsFixtureAgent) run(ctx context.Context, agent string) {
	for ctx.Err() == nil {
		var message wire.Message
		if a.socket.ReadJSON(&message) != nil {
			return
		}
		switch message.Type {
		case "state.sync":
			var config wire.Config
			if wire.Decode(message.Data, &config) != nil {
				return
			}
			a.mu.Lock()
			a.jobs = slices.Clone(config.DDNS)
			a.mu.Unlock()
			if a.send(wire.New("state.accepted", "", message.Revision, map[string]any{})) != nil || a.send(wire.New("state.applied", "", message.Revision, map[string]any{})) != nil {
				return
			}
			if a.send(wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: agent, BootID: fixtureBoot, ConfigRevision: message.Revision, Time: time.Now(), Ready: true, Synchronized: true, Users: wire.UserMetrics{}})) != nil {
				return
			}
		case "ddns.changes.ack":
			var ack wire.DDNSChangeAck
			if wire.Decode(message.Data, &ack) != nil {
				return
			}
			for _, id := range ack.IDs {
				select {
				case a.acks <- id:
				case <-ctx.Done():
					return
				}
			}
		case "command":
			var request struct {
				Name string          `json:"name"`
				Args json.RawMessage `json:"args"`
			}
			if wire.Decode(message.Data, &request) != nil {
				return
			}
			data := any(map[string]any{})
			if request.Name == "firewall.protected.sync" {
				var args struct {
					IPs []string `json:"ips"`
				}
				if wire.Decode(request.Args, &args) != nil {
					return
				}
				a.mu.Lock()
				a.protected = slices.Clone(args.IPs)
				held := a.hold && slices.Contains(args.IPs, "9.9.9.9")
				if held {
					a.pending = append(a.pending, message)
				}
				a.mu.Unlock()
				if held {
					continue
				}
			}
			if request.Name == "traffic.flush" {
				data = wire.TrafficBarrier{AgentID: agent, Through: time.Now()}
			}
			if a.result(message, request.Name, data) != nil {
				return
			}
		}
	}
}
