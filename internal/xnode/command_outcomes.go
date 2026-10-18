package xnode

import (
	"context"
	"encoding/json"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type CommandOutcome string

const (
	CommandUndelivered CommandOutcome = "undelivered"
	CommandRejected    CommandOutcome = "rejected"
	CommandFailed      CommandOutcome = "failed"
	CommandUncertain   CommandOutcome = "uncertain"
)

// CommandError reports delivery/execution certainty without exposing agent logs.
type CommandError struct {
	Outcome CommandOutcome
	Cause   error
}

func (e *CommandError) Error() string { return "native command " + string(e.Outcome) }
func (e *CommandError) Unwrap() error { return e.Cause }

func commandDeadline(name string) time.Duration {
	return wire.CommandTimeout(name)
}

func waitCommand(ctx context.Context, peer *agentPeer, pending <-chan wire.Message, id, name string, delivery <-chan time.Time) (json.RawMessage, error) {
	var execution <-chan time.Time
	var timer *time.Timer
	accepted := false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil, &CommandError{CommandUncertain, ctx.Err()}
		case <-peer.ctx.Done():
			return nil, &CommandError{CommandUncertain, ErrUnavailable}
		case <-delivery:
			return nil, &CommandError{CommandUncertain, context.DeadlineExceeded}
		case <-execution:
			return nil, &CommandError{CommandUncertain, context.DeadlineExceeded}
		case message := <-pending:
			if message.Type == "error" {
				if accepted {
					return nil, &CommandError{CommandUncertain, ErrUnavailable}
				}
				return nil, &CommandError{CommandRejected, ErrUnavailable}
			}
			if message.Type == "command.accepted" {
				var frame struct {
					Command string `json:"command"`
				}
				if wire.Decode(message.Data, &frame) != nil || frame.Command != name {
					return nil, &CommandError{CommandUncertain, ErrInvalid}
				}
				delivery = nil
				accepted = true
				if timer == nil {
					timer = time.NewTimer(commandDeadline(name))
					execution = timer.C
				}
				continue
			}
			var result struct {
				ID       string          `json:"id"`
				Command  string          `json:"command"`
				Status   string          `json:"status"`
				Data     json.RawMessage `json:"data"`
				Error    string          `json:"error"`
				Finished time.Time       `json:"finished"`
			}
			if message.Type != "command.result" || wire.Decode(message.Data, &result) != nil || result.ID != id || result.Command != name {
				return nil, &CommandError{CommandUncertain, ErrInvalid}
			}
			switch result.Status {
			case "completed":
				return result.Data, nil
			case "failed":
				return nil, &CommandError{CommandFailed, ErrUnavailable}
			default:
				return nil, &CommandError{CommandUncertain, ErrUnavailable}
			}
		}
	}
}
