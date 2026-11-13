package connections

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Service) currentAdministrator(ctx context.Context, actor model.User) error {
	if actor.ID == "" || actor.AccountGeneration < 0 {
		return ErrScanForbidden
	}
	current, err := s.repository.UserByID(ctx, actor.ID)
	if err != nil {
		return errors.Join(ErrScanUnavailable, err)
	}
	if current.ID != actor.ID || current.AccountGeneration != actor.AccountGeneration ||
		current.Role != "admin" || current.AccountStatus != "active" || current.BotBlocked {
		return ErrScanForbidden
	}
	return nil
}
