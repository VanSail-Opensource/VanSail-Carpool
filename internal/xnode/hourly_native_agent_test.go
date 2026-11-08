package xnode_test

import (
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (a *hourlyNativeAgent) send(message wire.Message) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if err := a.socket.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	return a.socket.WriteJSON(message)
}

func (a *hourlyNativeAgent) report() error {
	a.mu.Lock()
	revision := a.revision
	a.mu.Unlock()
	if revision == 0 {
		return nil
	}
	return a.send(wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: fixtureAgent, BootID: fixtureBoot, ConfigRevision: revision, Time: time.Now(), Ready: true, Synchronized: true, Users: wire.UserMetrics{}}))
}

func (a *hourlyNativeAgent) telemetry() {
	defer close(a.telemetryDone)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-tick.C:
			if a.report() != nil {
				return
			}
		}
	}
}

func (a *hourlyNativeAgent) run() {
	defer close(a.done)
	for {
		var message wire.Message
		if a.socket.ReadJSON(&message) != nil {
			return
		}
		if message.Type != "state.sync" {
			continue
		}
		var config wire.Config
		if wire.Decode(message.Data, &config) != nil {
			return
		}
		a.mu.Lock()
		a.config, a.revision = config, message.Revision
		a.mu.Unlock()
		if a.send(wire.New("state.accepted", "", message.Revision, map[string]any{})) != nil || a.send(wire.New("state.applied", "", message.Revision, map[string]any{})) != nil || a.report() != nil {
			return
		}
	}
}
