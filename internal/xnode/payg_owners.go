package xnode

import "context"

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
