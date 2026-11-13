package xnode

import "context"

func (s *Service) SaveNamedServiceAs(ctx context.Context, actor, id, name string, access ServiceAccess) (NamedService, error) {
	if actor == "" {
		return NamedService{}, ErrInvalid
	}
	return s.saveNamedService(ctx, id, name, access, actor)
}

func (s *Service) persistNamedService(ctx context.Context, actor string, identity Identity, access ServiceAccess) (Identity, error) {
	if actor != "" {
		writer, ok := s.repo.(interface {
			SaveAdminXNodeNamedService(context.Context, string, Identity, ServiceAccess) (Identity, error)
		})
		if !ok {
			return identity, ErrUnavailable
		}
		return writer.SaveAdminXNodeNamedService(ctx, actor, identity, access)
	}
	writer, ok := s.repo.(interface {
		SaveXNodeNamedService(context.Context, Identity, ServiceAccess) (Identity, error)
	})
	if !ok {
		return identity, ErrUnavailable
	}
	return writer.SaveXNodeNamedService(ctx, identity, access)
}
