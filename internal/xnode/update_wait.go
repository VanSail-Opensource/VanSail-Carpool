package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrUpdateNotReady = errors.New("X-Node update is confirmed but readiness is pending")

// Recovery reads the durable native transaction, never the current release feed.
// Every status request is bound to the peer used for its readiness observation.
func (s *Service) waitUpdate(ctx context.Context, node string, expected UpdateExpectation) (json.RawMessage, error) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var last json.RawMessage
	matchingConfirmation := false
	for {
		if s.master.Connected(node) && containsCapability(s.master.Capabilities(node), "update_transaction_v1") {
			child, cancel := context.WithTimeout(ctx, 5*time.Second)
			transaction, peer, err := s.master.updateTransaction(child, node)
			cancel()
			if err == nil {
				last, err = json.Marshal(transaction)
				if err != nil {
					return nil, &CommandError{CommandUncertain, ErrUpdateEvidence}
				}
				if expected.matches(transaction) {
					if transaction.Status == "failed" || (expected.Command == "update.install" && transaction.Status == "rolled_back") {
						return last, &CommandError{CommandFailed, ErrUpdateEvidence}
					}
					matchingConfirmation = expected.confirmed(transaction)
					if matchingConfirmation && s.master.updateReadyPeer(node, expected.runningVersion(), peer) {
						return last, nil
					}
				} else {
					matchingConfirmation = false
				}
			}
		}
		select {
		case <-ctx.Done():
			cause := ErrUpdateEvidence
			if matchingConfirmation {
				cause = ErrUpdateNotReady
			}
			return last, &CommandError{CommandUncertain, errors.Join(cause, ctx.Err())}
		case <-tick.C:
		}
	}
}

func (m *Master) updateReady(node, version string) bool {
	return m.updateReadyPeer(node, version, nil)
}

func (m *Master) updateReadyPeer(node, version string, expected *agentPeer) bool {
	m.mu.RLock()
	snapshot := m.live[node]
	peer := m.peers[node]
	m.mu.RUnlock()
	if peer == nil || (expected != nil && peer != expected) {
		return false
	}
	peer.mu.Lock()
	revision := peer.revision
	peer.mu.Unlock()
	return version != "" && snapshot.hello.Version == version && !snapshot.hello.UpgradePending && snapshot.report.Ready &&
		snapshot.report.Synchronized && revision > 0 && snapshot.report.ConfigRevision >= revision && freshSnapshot(snapshot, m.service.now(), 5)
}
