package xnode_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

type routingFrame struct {
	node, phase string
	transit     int
}

func routingAgent(ctx context.Context, f fixture, node, agent string, socket *websocket.Conn, hold string, release <-chan struct{}, frames chan<- routingFrame, blocked chan<- struct{}, errs chan<- error) {
	_ = socket.SetReadDeadline(time.Time{})
	send := func(m wire.Message) bool {
		if err := socket.WriteJSON(m); err != nil {
			select {
			case errs <- err:
			default:
			}
			return false
		}
		return true
	}
	for {
		var message wire.Message
		if err := socket.ReadJSON(&message); err != nil {
			return
		}
		switch message.Type {
		case "state.sync":
			var config wire.Config
			if err := wire.Decode(message.Data, &config); err != nil {
				errs <- err
				return
			}
			plan, err := f.store.XNodeRoutingPlan(ctx, f.identity.ProxyUserID)
			phase := "initial"
			if err == nil {
				phase = plan.Phase
			}
			count := 0
			for _, u := range config.Users {
				count += len(u.TransitCredentials)
			}
			select {
			case frames <- routingFrame{node, phase, count}:
			case <-ctx.Done():
				return
			}
			if node == hold && phase == "applying" {
				select {
				case blocked <- struct{}{}:
				default:
				}
				select {
				case <-release:
				case <-ctx.Done():
					return
				}
			}
			if !send(wire.New("state.accepted", "", message.Revision, map[string]any{})) || !send(wire.New("state.applied", "", message.Revision, map[string]any{})) {
				return
			}
			if !send(wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: agent, BootID: fixtureBoot, ConfigRevision: message.Revision, Time: time.Now(), Ready: true, Synchronized: true, Users: wire.UserMetrics{}})) {
				return
			}
		case "command":
			var request struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(message.Data, &request) != nil {
				errs <- fmt.Errorf("invalid command")
				return
			}
			data := any(map[string]any{})
			if request.Name == "config.validate" {
				// A slow native validation must not monopolize unrelated member
				// mutations while the caller waits for this agent's reply.
				ready := make(chan error, 1)
				go func() { _, err := f.service.EnsureIdentity(ctx, f.user); ready <- err }()
				select {
				case err := <-ready:
					if err != nil {
						errs <- err
					}
				case <-time.After(2 * time.Second):
					errs <- fmt.Errorf("routing preflight held the global mutation mutex")
				}
			}
			if request.Name == "traffic.flush" {
				data = wire.TrafficBarrier{AgentID: agent}
			}
			if !send(wire.New("command.result", message.ID, 0, map[string]any{"id": message.ID, "command": request.Name, "status": "completed", "data": data, "finished": time.Now()})) {
				return
			}
		}
	}
}
