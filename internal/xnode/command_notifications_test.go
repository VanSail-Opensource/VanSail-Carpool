package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
)

func TestTerminalCommandResultSurvivesAcceptanceFlood(t *testing.T) {
	pending := make(chan wire.Message, 4)
	for range 1000 {
		offerCommandFrame(pending, wire.New("command.accepted", "receipt", 0, map[string]string{"command": "runtime.reload"}))
	}
	result := wire.New("command.result", "receipt", 0, map[string]any{"id": "receipt", "command": "runtime.reload", "status": "completed", "data": map[string]bool{"applied": true}})
	offerCommandFrame(pending, result)
	for range 1000 {
		offerCommandFrame(pending, wire.New("command.accepted", "receipt", 0, nil))
	}
	_, err := waitCommand(context.Background(), &agentPeer{ctx: context.Background()}, pending, "receipt", "runtime.reload", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOnlyDefiniteWorkerBusyRejectionIsRetryable(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		pending := make(chan wire.Message, 2)
		if accepted {
			offerCommandFrame(pending, wire.New("command.accepted", "receipt", 0, map[string]string{"command": "runtime.reload"}))
		}
		offerCommandFrame(pending, wire.New("error", "receipt", 0, map[string]string{"error": "utility workers are busy"}))
		_, err := waitCommand(context.Background(), &agentPeer{ctx: context.Background()}, pending, "receipt", "runtime.reload", nil)
		if errors.Is(err, outbox.ErrBusy) == accepted {
			t.Fatalf("execution certainty lost: %t %v", accepted, err)
		}
		var outcome *CommandError
		if accepted && (!errors.As(err, &outcome) || outcome.Outcome != CommandUncertain) {
			t.Fatal("queued acceptance was lost before command error", err)
		}
	}
}
