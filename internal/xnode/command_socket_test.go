package xnode_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestAcceptedRecoveryCommandDoesNotBlockAccountingAcknowledgments(t *testing.T) {
	f := controlFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0)
	if err := socket.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	readFrame(t, socket, "state.sync")
	finished := make(chan error, 1)
	go func() {
		_, err := f.master.Command(ctx, f.node.ID, "recovery", "runtime.reload", map[string]any{})
		finished <- err
	}()
	command := readFrame(t, socket, "command")
	if command.ID != "recovery" {
		t.Fatal(command)
	}
	if err := socket.WriteJSON(wire.New("command.accepted", command.ID, 0, map[string]string{"command": "runtime.reload"})); err != nil {
		t.Fatal(err)
	}
	report := wire.TrafficReport{ID: fixtureAgent + ":1", Traffic: wire.Traffic{}}
	if err := socket.WriteJSON(wire.New("traffic.report", report.ID, 0, report)); err != nil {
		t.Fatal(err)
	}
	readFrame(t, socket, "traffic.ack")
	select {
	case err := <-finished:
		t.Fatalf("acceptance completed command early: %v", err)
	default:
	}
	if err := socket.WriteJSON(wire.New("command.result", command.ID, 0, map[string]any{"id": command.ID, "command": "runtime.reload", "status": "completed", "finished": time.Now().UTC(), "data": map[string]bool{"applied": true}})); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
