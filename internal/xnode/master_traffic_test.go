package xnode

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type rejectedTrafficRepository struct {
	Repository
	calls int
}

func (r *rejectedTrafficRepository) RecordXNodeTraffic(context.Context, string, string, uint64, string, wire.TrafficReport) error {
	r.calls++
	return ErrTrafficSequenceGap
}

func TestRejectedTrafficBackoffKeepsSocketReaderAndRecoveryAvailable(t *testing.T) {
	repo := &rejectedTrafficRepository{}
	service := NewService(repo, nil, nil, nil, nil)
	now := time.Now()
	service.now = func() time.Time { return now }
	master := NewMaster(service, nil)
	pending := make(chan wire.Message, 1)
	peer := &agentPeer{agentID: "agent", paygReady: true, paygRecoveryQueued: true, pending: map[string]chan wire.Message{"command": pending}}
	message := wire.New("traffic.report", "agent:2", 0, wire.TrafficReport{ID: "agent:2"})
	for attempt := range 10 {
		if err := master.receive(context.Background(), peer, message); !errors.Is(err, ErrConflict) {
			t.Fatal("journal conflict lost", err)
		}
		for range 100 {
			if err := master.receive(context.Background(), peer, message); err != nil {
				t.Fatal("unchanged retry repeated work", err)
			}
		}
		if repo.calls != attempt+1 || peer.paygReady || !peer.paygRecoveryQueued {
			t.Fatal("retry repeated a commit or discarded recovery", repo.calls, peer)
		}
		if peer.trafficRetry.delay > 30*time.Second {
			t.Fatal("retry interval exceeded bound")
		}
		now = peer.trafficRetry.after
	}
	// No queue or socket is attached: a false ACK would panic above. Pending
	// command results must still reach waiters while this journal is blocked.
	if err := master.receive(context.Background(), peer, wire.New("command.result", "command", 0, nil)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pending:
	default:
		t.Fatal("journal backoff blocked a command result")
	}
	changed := wire.New("traffic.report", "agent:1", 0, wire.TrafficReport{ID: "agent:1"})
	if err := master.receive(context.Background(), peer, changed); !errors.Is(err, ErrTrafficSequenceGap) || repo.calls != 11 || peer.trafficRetry.delay != time.Second {
		t.Fatal("missing head was suppressed by a later report", err, repo.calls)
	}
}

func TestTrafficConflictDiagnosticsPreserveConflictMatching(t *testing.T) {
	for _, err := range []error{ErrTrafficAgentMismatch, ErrTrafficDigestMismatch, ErrTrafficSequenceGap, ErrTrafficSampleMissing, ErrTrafficPeriodSettled, ErrTrafficOwnerMismatch} {
		wrapped := fmt.Errorf("private storage context: %w", err)
		if !errors.Is(wrapped, ErrConflict) || frameErrorCause(wrapped) != "state_conflict" || safeFrameErrorCode(wrapped) != err.Error() {
			t.Fatal("traffic diagnostic lost safe classification", err)
		}
	}
	err := fmt.Errorf("private storage context: %w", NewTrafficSequenceGap(12, 9))
	fields := frameErrorDetails(err)
	if !errors.Is(err, ErrTrafficSequenceGap) || safeFrameErrorCode(err) != ErrTrafficSequenceGap.Error() || len(fields) != 4 || fields[1] != uint64(12) || fields[3] != uint64(9) {
		t.Fatal("missing sanitized journal positions", fields)
	}
}
