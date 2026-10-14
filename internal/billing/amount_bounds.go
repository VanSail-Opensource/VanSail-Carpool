package billing

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const (
	defaultAddTXBMinimumMinor int64 = 100
	defaultAddTXBMaximumMinor int64 = 10_000_000_000
)

type amountBoundsReader interface {
	AddTXBBounds(context.Context) (model.AddTXBBounds, error)
}

func (s *Service) validateAddTXBAmount(ctx context.Context, amount int64) error {
	minimum, maximum := defaultAddTXBMinimumMinor, defaultAddTXBMaximumMinor
	if reader, ok := s.repository.(amountBoundsReader); ok {
		bounds, err := reader.AddTXBBounds(ctx)
		if err != nil {
			return fmt.Errorf("load Add TXB bounds: %w", err)
		}
		minimum, maximum = bounds.MinimumTXBMinor, bounds.MaximumTXBMinor
	}
	if amount < minimum || amount > maximum {
		return fmt.Errorf("%w: TXB amount is outside configured bounds", ErrInvalidOrder)
	}
	return nil
}
