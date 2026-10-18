package xnode

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) executeNodeCommand(ctx context.Context, node string, operation providerops.Operation, request CommandRequest, args map[string]any) (json.RawMessage, error) {
	update := request.Name == "update.install" || request.Name == "update.rollback"
	// An interrupted update is reconciled, never blindly repeated. Other exposed
	// commands set desired state or perform bounded reads/probes and are retry-safe.
	if update && operation.Attempts > 1 {
		return nil, s.waitUpdate(ctx, node, operation.Receipt.ID, request)
	}
	result, err := s.master.Command(ctx, node, operation.Receipt.ID, request.Name, args)
	if !update {
		if err == nil && (request.Name == "runtime.reload" || request.Name == "geodata.refresh") {
			var activation struct {
				Applied bool `json:"applied"`
				Ready   bool `json:"ready"`
			}
			if json.Unmarshal(result, &activation) != nil || !activation.Applied || !activation.Ready {
				return nil, &CommandError{CommandFailed, ErrUnavailable}
			}
		}
		return result, err
	}
	if outcome, ok := err.(*CommandError); ok && outcome.Outcome != CommandUncertain {
		return nil, err
	}
	if err == nil && request.Name == "update.install" {
		var release struct {
			Available bool   `json:"available"`
			Current   string `json:"current"`
			Version   string `json:"version"`
		}
		if json.Unmarshal(result, &release) == nil && !release.Available && s.master.updateReady(node, release.Current) {
			return result, nil
		}
	}
	if err := s.waitUpdate(ctx, node, operation.Receipt.ID, request); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) waitUpdate(ctx context.Context, node, id string, request CommandRequest) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		if s.master.Connected(node) && slices.Contains(s.master.Capabilities(node), "update_transaction_v1") {
			child, cancel := context.WithTimeout(ctx, 5*time.Second)
			raw, err := s.master.Command(child, node, "", "update.status", map[string]any{})
			cancel()
			if err == nil {
				var transaction struct {
					ID       string `json:"id"`
					Version  string `json:"version"`
					Previous string `json:"previous"`
					Status   string `json:"status"`
				}
				if json.Unmarshal(raw, &transaction) == nil {
					matched := request.Name == "update.rollback" || transaction.ID == id
					if matched {
						if request.Name == "update.install" && (transaction.Status == "rolled_back" || transaction.Status == "failed") {
							return &CommandError{CommandFailed, ErrUnavailable}
						}
						expected, complete := transaction.Version, transaction.Status == "confirmed"
						if request.Name == "update.rollback" {
							expected, complete = transaction.Previous, transaction.Status == "rolled_back"
						}
						if complete && s.master.updateReady(node, expected) {
							return nil
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return &CommandError{CommandUncertain, ctx.Err()}
		case <-tick.C:
		}
	}
}

func (m *Master) updateReady(node, version string) bool {
	m.mu.RLock()
	snapshot := m.live[node]
	peer := m.peers[node]
	m.mu.RUnlock()
	if peer == nil {
		return false
	}
	peer.mu.Lock()
	revision := peer.revision
	peer.mu.Unlock()
	return version != "" && snapshot.hello.Version == version && !snapshot.hello.UpgradePending && snapshot.report.Ready && snapshot.report.Synchronized && revision > 0 && snapshot.report.ConfigRevision >= revision && freshSnapshot(snapshot, m.service.now(), 5)
}
