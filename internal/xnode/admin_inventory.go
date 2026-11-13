package xnode

import "context"

func (s *Service) SaveGroupAs(ctx context.Context, actor string, item Group) (Group, error) {
	if actor == "" {
		return item, ErrInvalid
	}
	return s.saveGroup(ctx, item, actor)
}

func (s *Service) SaveEndpointAs(ctx context.Context, actor string, item Endpoint) (Endpoint, error) {
	if actor == "" {
		return item, ErrInvalid
	}
	return s.saveEndpoint(ctx, item, actor)
}

func (s *Service) DeleteGroupAs(ctx context.Context, actor, id string) error {
	if actor == "" {
		return ErrInvalid
	}
	return s.deleteGroup(ctx, id, actor)
}

func (s *Service) DeleteEndpointAs(ctx context.Context, actor, id string) error {
	if actor == "" {
		return ErrInvalid
	}
	writer, ok := s.repo.(interface {
		DeleteAdminXNodeEndpoint(context.Context, string, string) error
	})
	if !ok {
		return ErrUnavailable
	}
	return writer.DeleteAdminXNodeEndpoint(ctx, actor, id)
}

func (s *Service) persistGroup(ctx context.Context, actor string, item Group) (Group, error) {
	if actor != "" {
		writer, ok := s.repo.(interface {
			SaveAdminXNodeGroup(context.Context, string, Group) (Group, error)
		})
		if !ok {
			return item, ErrUnavailable
		}
		return writer.SaveAdminXNodeGroup(ctx, actor, item)
	}
	return item, s.repo.SaveXNodeGroup(ctx, item)
}

func (s *Service) persistEndpoint(ctx context.Context, actor string, item Endpoint) (Endpoint, error) {
	if actor != "" {
		writer, ok := s.repo.(interface {
			SaveAdminXNodeEndpoint(context.Context, string, Endpoint) (Endpoint, error)
		})
		if !ok {
			return item, ErrUnavailable
		}
		return writer.SaveAdminXNodeEndpoint(ctx, actor, item)
	}
	return item, s.repo.SaveXNodeEndpoint(ctx, item)
}
