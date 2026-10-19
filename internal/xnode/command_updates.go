package xnode

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) executeNodeCommand(ctx context.Context, node string, operation providerops.Operation, request CommandRequest, args map[string]any) (json.RawMessage, error) {
	update := request.Name == "update.install" || request.Name == "update.rollback"
	// A rollback is confirmed by the running release changing away from the
	// version observed before this attempt. A retried attempt that cannot observe
	// the original version remains pending review instead of reporting success.
	previous := ""
	if update && request.Name == "update.rollback" && operation.Attempts <= 1 {
		previous = s.master.Version(node)
	}
	// An interrupted update is reconciled, never blindly repeated. Other exposed
	// commands set desired state or perform bounded reads/probes and are retry-safe.
	if update && operation.Attempts > 1 {
		return nil, s.waitUpdate(ctx, node, request, previous)
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
	if err := s.waitUpdate(ctx, node, request, previous); err != nil {
		return nil, err
	}
	return result, nil
}

// waitUpdate reconciles an interrupted upgrade through the documented
// update.check command. The pinned agent publishes the running and available
// releases, not an internal transaction identifier, so completion is derived
// from the release the agent reports together with its synchronized hello.
func (s *Service) waitUpdate(ctx context.Context, node string, request CommandRequest, previous string) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		if s.master.Connected(node) {
			child, cancel := context.WithTimeout(ctx, 5*time.Second)
			raw, err := s.master.Command(child, node, "", "update.check", map[string]any{})
			cancel()
			if err == nil {
				var release struct {
					Current   string `json:"current"`
					Available bool   `json:"available"`
				}
				if json.Unmarshal(raw, &release) == nil && release.Current != "" && s.master.updateReady(node, release.Current) {
					if request.Name == "update.rollback" {
						if previous != "" && release.Current != previous {
							return nil
						}
					} else if request.Version != "" {
						if release.Current == request.Version {
							return nil
						}
					} else if !release.Available {
						return nil
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
