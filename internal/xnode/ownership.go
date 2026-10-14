package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

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
