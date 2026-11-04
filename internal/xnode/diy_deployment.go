package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// Client-only saves finish transactionally. Retired whitelist receipts cannot recreate policies.
func (s *Service) runDIYOperation(ctx context.Context, operation providerops.Operation) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	repo, ok := s.repo.(operationRepository)
	if !ok || s.master == nil {
		return ErrUnavailable
	}
	items, err := repo.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if len(items) != 1 || items[0].TargetType != "xnode_user" {
		return ErrInvalid
	}
	return s.runRoutingOperation(ctx, operation)
}
