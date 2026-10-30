package billing

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

// BuiltinUSDT supplies local availability and pricing without a provider profile.
type BuiltinUSDT interface {
	Methods(context.Context) ([]model.PaymentMethod, error)
	Expiry(context.Context) (time.Duration, error)
}

// SetBuiltinUSDT is called during bootstrap before serving requests.
func (s *Service) SetBuiltinUSDT(provider BuiltinUSDT) { s.builtin = provider }

func (s *Service) builtinEnabled(ctx context.Context, rail string) (bool, error) {
	if s.builtin == nil {
		return false, nil
	}
	methods, err := s.builtin.Methods(ctx)
	if err != nil {
		return false, err
	}
	for _, method := range methods {
		if method.Rail == rail {
			return method.Available, nil
		}
	}
	return false, nil
}

func builtinCreateRejection(provider string, err error) (string, bool) {
	if provider != usdt.Provider {
		return "", false
	}
	for _, known := range []error{usdt.ErrCapacity, usdt.ErrUnavailable, usdt.ErrConfiguration, usdt.ErrConflict} {
		if errors.Is(err, known) {
			return known.Error(), true
		}
	}
	return "", false
}
