package xnode

import "context"

func (s *Service) observedPrincipalOwners(ctx context.Context, ids []int) (map[int]string, error) {
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	if r, ok := s.repo.(interface {
		XNodeObservedPrincipalOwners(context.Context, []int) (map[int]string, error)
	}); ok {
		return r.XNodeObservedPrincipalOwners(ctx, ids)
	}
	return s.principalOwners(ctx)
}

func (s *Service) principalOwners(ctx context.Context) (map[int]string, error) {
	if r, ok := s.repo.(interface {
		XNodePrincipalOwners(context.Context) (map[int]string, error)
	}); ok {
		return r.XNodePrincipalOwners(ctx)
	}
	items, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, i := range items {
		if owner := i.MemberOwnerID(); owner != "" {
			out[int(i.ProxyUserID)] = owner
		}
	}
	return out, nil
}
