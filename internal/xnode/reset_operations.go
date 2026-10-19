package xnode

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

func (s *Service) ResetPurchaseTrafficForOperation(ctx context.Context, id, purchase, operation string) error {
	if purchase == "" || !validResetOperation(operation) {
		return ErrInvalid
	}
	return s.resetTraffic(ctx, id, purchase, operation)
}

func (s *Service) ResetOperationCompleted(ctx context.Context, id, purchase, operation string) (bool, error) {
	if purchase == "" || !validResetOperation(operation) {
		return false, ErrInvalid
	}
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return false, err
	}
	return s.resetOperationCompleted(ctx, identity, purchase, operation)
}

func (s *Service) resetOperationCompleted(ctx context.Context, identity Identity, purchase, operation string) (bool, error) {
	period, err := s.repo.XNodePeriod(ctx, operation)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if period.ProxyUserID != identity.ProxyUserID || period.PurchaseID != purchase || period.LastResetAt == nil {
		return false, ErrConflict
	}
	return true, nil
}

func validResetOperation(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
