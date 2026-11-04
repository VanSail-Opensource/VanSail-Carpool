package xnode

import "context"

// NativeOwner verifies that the node belongs to this control plane.
func (s *Service) NativeOwner(ctx context.Context, nodeID string) (bool, error) {
	node, err := s.repo.XNodeNode(ctx, nodeID)
	if isMissing(err) {
		return false, nil
	}
	return err == nil && node.RetiredAt == nil, err
}
