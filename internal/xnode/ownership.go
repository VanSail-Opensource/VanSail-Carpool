package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Legacy ownership keeps listeners, users and operational host policy empty.
// Stage shared geodata first so config.validate can check the future candidate
// against real local files before the source node is disabled.
func (s *Service) legacyConfiguration(ctx context.Context) (wire.Config, error) {
	global, err := s.Global(ctx)
	if err != nil {
		return wire.Config{}, err
	}
	native := map[string]any{"inbounds": []any{}, "outbounds": []any{map[string]any{"tag": "direct", "protocol": "freedom"}}}
	_, hashes, err := GlobalWire(global, native)
	if err != nil {
		return wire.Config{}, err
	}
	raw, err := json.Marshal(native)
	if err != nil {
		return wire.Config{}, err
	}
	config := wire.Config{Xray: raw, Users: []wire.User{}, Telemetry: global.Telemetry, AssetHashes: hashes}
	return config, config.Validate()
}

func (s *Service) NativeOwner(ctx context.Context, nodeID string) (bool, error) {
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return false, err
	}
	for _, node := range state.Nodes {
		if node.NodeID == nodeID {
			return node.Phase == "native", nil
		}
	}
	_, err = s.repo.XNodeNode(ctx, nodeID)
	if isMissing(err) {
		return false, nil
	}
	return err == nil, err
}

// Candidate expands a node's future native configuration for listener-free
// preflight. It does not transfer ownership or write desired state.
func (s *Service) Candidate(ctx context.Context, nodeID string) (wire.Config, error) {
	repository := candidateRepository{Repository: s.repo, nodeID: nodeID}
	service := NewService(repository, s.settings, s.vault, s.publicURL, s.validationQueue)
	service.master = s.master
	service.now = s.now
	service.policyAssets = s.policyAssets
	service.routingAssets, service.routingRuntime = s.routingAssets, s.routingRuntime
	return service.Compile(ctx, nodeID)
}

type candidateRepository struct {
	Repository
	nodeID string
}

func (r candidateRepository) XNodeMigration(ctx context.Context) (MigrationState, error) {
	state, err := r.Repository.XNodeMigration(ctx)
	if err != nil {
		return state, err
	}
	for i := range state.Nodes {
		if state.Nodes[i].NodeID == r.nodeID {
			state.Nodes[i].Phase = "native"
		}
	}
	return state, nil
}
