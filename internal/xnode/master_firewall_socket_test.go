package xnode_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func TestFirewallConfirmationRecoversDroppedPongAndLostReply(t *testing.T) {
	for _, mode := range []string{"dropped-pong", "lost-reply"} {
		t.Run(mode, func(t *testing.T) {
			f := controlFixture(t, false)
			server := httptest.NewServer(f.master)
			defer server.Close()
			_, token, err := f.service.Enrollment(context.Background(), f.node.ID)
			if err != nil {
				t.Fatal(err)
			}
			socket := agentSocket(t, server, token, fixtureAgent, 0)
			_ = socket.SetReadDeadline(time.Now().Add(35 * time.Second))
			if err = socket.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
				t.Fatal(err)
			}
			readFrame(t, socket, "state.sync")
			pings := 0
			socket.SetPingHandler(func(payload string) error {
				pings++
				if mode == "dropped-pong" && pings == 1 {
					return nil
				}
				return socket.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(time.Second))
			})
			report := wire.TelemetryReport{AgentID: fixtureAgent, BootID: fixtureBoot, Time: time.Now(), Ready: true, Synchronized: true}
			if err = socket.WriteJSON(wire.New("telemetry.report", "", 0, report)); err != nil {
				t.Fatal(err)
			}
			if err = socket.WriteJSON(wire.New("firewall.pending", "", 0, map[string]string{"id": "transaction-1", "status": "pending"})); err != nil {
				t.Fatal(err)
			}
			confirmed := false
			for {
				message := readFrame(t, socket, "command")
				var command struct {
					Name string          `json:"name"`
					Args json.RawMessage `json:"args"`
				}
				if err = wire.Decode(message.Data, &command); err != nil {
					t.Fatal(err)
				}
				var data any
				switch command.Name {
				case "firewall.confirm":
					confirmed = true
					if mode == "lost-reply" {
						continue
					}
					data = map[string]bool{"confirmed": true}
				case "firewall.status":
					status := "pending"
					if confirmed {
						status = "confirmed"
					}
					data = map[string]any{"id": "transaction-1", "status": status, "policy": map[string]any{}}
				default:
					continue
				}
				result := map[string]any{"id": message.ID, "command": command.Name, "status": "completed", "finished": time.Now(), "data": data}
				if err = socket.WriteJSON(wire.New("command.result", message.ID, 0, result)); err != nil {
					t.Fatal(err)
				}
				if confirmed {
					break
				}
			}
			if mode == "dropped-pong" && pings < 2 {
				t.Fatalf("confirmation did not retry ping: %d", pings)
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				metrics, err := f.master.Metrics(context.Background())
				if err == nil && len(metrics.Nodes) > 0 && metrics.Nodes[0].Synchronized {
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
			t.Fatal("confirmed firewall remains pending")
		})
	}
}
