package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type memberIdentityRepository interface {
	SaveXNodeMemberIdentity(context.Context, Identity, Identity, int64) (Identity, error)
}

func (s *Service) memberIdentityRepository() (memberIdentityRepository, bool) {
	source := s.repo
	if reads, ok := source.(*compileReads); ok {
		source = reads.Repository
	}
	repository, ok := source.(memberIdentityRepository)
	return repository, ok
}

func (s *Service) ensureMemberIdentity(ctx context.Context, user model.User) (Identity, error) {
	repository, ok := s.memberIdentityRepository()
	if !ok {
		return Identity{}, ErrUnavailable
	}
	return s.ensureIdentityWithPersistence(ctx, user, func(expected, next Identity) (Identity, error) {
		return repository.SaveXNodeMemberIdentity(ctx, expected, next, user.AccountGeneration)
	})
}
