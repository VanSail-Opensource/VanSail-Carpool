package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestCommandResultsKeepDeliveryAndExecutionOutcomesDistinct(t *testing.T) {
	for _, tc := range []struct {
		status  string
		outcome CommandOutcome
	}{{"completed", ""}, {"failed", CommandFailed}, {"cancelled", CommandUncertain}} {
		pending := make(chan wire.Message, 2)
		pending <- wire.New("command.accepted", "job", 0, map[string]string{"command": "runtime.reload"})
		pending <- wire.New("command.result", "job", 0, map[string]any{"id": "job", "command": "runtime.reload", "status": tc.status, "data": map[string]bool{"applied": true}, "error": "redacted", "finished": time.Now().UTC()})
		raw, err := waitCommand(context.Background(), &agentPeer{ctx: context.Background()}, pending, "job", "runtime.reload", nil)
		if tc.outcome == "" {
			if err != nil || len(raw) == 0 {
				t.Fatal(raw, err)
			}
			continue
		}
		var outcome *CommandError
		if !errors.As(err, &outcome) || outcome.Outcome != tc.outcome {
			t.Fatalf("%s result: %v", tc.status, err)
		}
	}
}

func TestRejectedDeliveryAndDisconnectedCommands(t *testing.T) {
	pending := make(chan wire.Message, 1)
	pending <- wire.New("error", "job", 0, map[string]string{"error": "busy"})
	_, err := waitCommand(context.Background(), &agentPeer{ctx: context.Background()}, pending, "job", "runtime.reload", nil)
	var outcome *CommandError
	if !errors.As(err, &outcome) || outcome.Outcome != CommandRejected {
		t.Fatal(err)
	}
	deadline := make(chan time.Time, 1)
	deadline <- time.Now()
	_, err = waitCommand(context.Background(), &agentPeer{ctx: context.Background()}, nil, "job", "runtime.reload", deadline)
	if !errors.As(err, &outcome) || outcome.Outcome != CommandUncertain {
		t.Fatal(err)
	}
	master := &Master{peers: map[string]*agentPeer{}}
	_, err = master.Command(context.Background(), "offline", "job", "runtime.reload", map[string]any{})
	if !errors.As(err, &outcome) || outcome.Outcome != CommandUndelivered {
		t.Fatal(err)
	}
}
