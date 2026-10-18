package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	jobqueue "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestLostUpgradeResultReconcilesTransactionAfterReconnect(t *testing.T) {
	f := controlFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0, "update_transaction_v1")
	if err := socket.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	baseline := readFrame(t, socket, "state.sync")
	receipt, err := f.service.QueueCommand(ctx, f.user.ID, f.node.ID, "install-reconnect", xnode.CommandRequest{Name: "update.install"})
	if err != nil {
		t.Fatal(err)
	}
	claimed := false
	for range 20 {
		job, err := f.store.ClaimOutboxJob(ctx, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if job == nil {
			break
		}
		var target struct {
			ID string `json:"operationId"`
		}
		_ = json.Unmarshal([]byte(job.Payload), &target)
		if target.ID == receipt.ID {
			operation, err := f.store.ProviderOperationByID(ctx, receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.service.HandleProviderOperation(ctx, operation, *job); !errors.Is(err, jobqueue.ErrDeferred) {
				t.Fatal(err)
			}
			claimed = true
			break
		}
		if err := f.store.CompleteOutboxJob(ctx, job.ID, job.Attempts, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if !claimed {
		t.Fatal("upgrade job missing")
	}
	command := readFrame(t, socket, "command")
	if command.ID != receipt.ID {
		t.Fatal("wrong durable command", command)
	}
	if err := socket.WriteJSON(wire.New("command.accepted", command.ID, 0, map[string]string{"command": "update.install"})); err != nil {
		t.Fatal(err)
	}
	_ = socket.Close()
	reconnected := agentSocket(t, server, token, fixtureAgent, baseline.Revision, "update_transaction_v1")
	if err := reconnected.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	var status wire.Message
	synchronized := false
	for status.ID == "" || !synchronized {
		var frame wire.Message
		if err := reconnected.ReadJSON(&frame); err != nil {
			t.Fatal(err)
		}
		switch frame.Type {
		case "state.sync":
			if err := reconnected.WriteJSON(wire.New("state.applied", frame.ID, frame.Revision, map[string]bool{"ready": true})); err != nil {
				t.Fatal(err)
			}
			if err := reconnected.WriteJSON(wire.New("telemetry.report", "", frame.Revision, wire.TelemetryReport{AgentID: fixtureAgent, BootID: fixtureBoot, Time: time.Now().UTC(), Ready: true, Synchronized: true, ConfigRevision: frame.Revision})); err != nil {
				t.Fatal(err)
			}
			synchronized = true
		case "command":
			status = frame
		}
	}
	var request struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(status.Data, &request) != nil || request.Name != "update.status" {
		t.Fatal("update was repeated instead of reconciled", status)
	}
	if err := reconnected.WriteJSON(wire.New("command.result", status.ID, 0, map[string]any{"id": status.ID, "command": "update.status", "status": "completed", "finished": time.Now().UTC(), "data": map[string]string{"id": receipt.ID, "status": "confirmed", "version": "fixture", "previous": "old"}})); err != nil {
		t.Fatal(err)
	}
	for {
		operation, err := f.store.ProviderOperationByID(ctx, receipt.ID)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Receipt.Status == "succeeded" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("reconnected upgrade was not confirmed", operation.Receipt, ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}
