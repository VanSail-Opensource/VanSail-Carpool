package xnode

import "context"

func (s *Service) EnrollmentAs(ctx context.Context, actor, id string) (string, string, error) {
	if actor == "" {
		return "", "", ErrInvalid
	}
	return s.enrollment(ctx, id, actor)
}

func (s *Service) DeleteNodeAs(ctx context.Context, actor, id string) error {
	if actor == "" {
		return ErrInvalid
	}
	if s.master != nil {
		return s.master.deleteNodeAs(ctx, id, actor)
	}
	return s.persistNodeDeletion(ctx, actor, id)
}

func (s *Service) RevokeSubscriptionAs(ctx context.Context, actor, id string) (string, error) {
	if actor == "" {
		return "", ErrInvalid
	}
	return s.revokeSubscription(ctx, id, actor)
}

func (s *Service) persistEnrollment(ctx context.Context, actor, id string, hash []byte) error {
	if actor != "" {
		writer, ok := s.repo.(interface {
			RotateAdminXNodeToken(context.Context, string, string, []byte) error
		})
		if !ok {
			return ErrUnavailable
		}
		return writer.RotateAdminXNodeToken(ctx, actor, id, hash)
	}
	return s.repo.RotateXNodeToken(ctx, id, hash)
}

func (s *Service) persistNodeDeletion(ctx context.Context, actor, id string) error {
	if actor != "" {
		writer, ok := s.repo.(interface {
			DeleteAdminXNodeNode(context.Context, string, string) error
		})
		if !ok {
			return ErrUnavailable
		}
		return writer.DeleteAdminXNodeNode(ctx, actor, id)
	}
	return s.repo.DeleteXNodeNode(ctx, id)
}

func (s *Service) persistSubscriptionRotation(ctx context.Context, actor string, identity Identity, hash []byte, cipher string) (Identity, error) {
	if actor != "" {
		writer, ok := s.repo.(interface {
			RotateAdminXNodeSubscription(context.Context, string, Identity, []byte, string) (Identity, error)
		})
		if !ok {
			return identity, ErrUnavailable
		}
		return writer.RotateAdminXNodeSubscription(ctx, actor, identity, hash, cipher)
	}
	writer, ok := s.repo.(interface {
		RotateXNodeSubscription(context.Context, Identity, []byte, string) (Identity, error)
	})
	if !ok {
		return identity, ErrUnavailable
	}
	return writer.RotateXNodeSubscription(ctx, identity, hash, cipher)
}
