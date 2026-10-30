package usdt

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

// Expiry provides the server-owned quote window to billing.
func (s *Service) Expiry(ctx context.Context) (time.Duration, error) {
	c, err := s.config(ctx)
	return time.Duration(c.ExpiryMinutes) * time.Minute, err
}

// Methods is appended independently of external payment-profile discovery.
func (s *Service) Methods(ctx context.Context) ([]model.PaymentMethod, error) {
	c, err := s.config(ctx)
	if err != nil {
		return nil, err
	}
	result := []model.PaymentMethod{}
	if !c.Enabled {
		return result, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, spec := range Registry {
		var n NetworkConfig
		for _, configured := range c.Networks {
			if configured.Network == spec.Network {
				n = configured
				break
			}
		}
		if !n.Enabled || !currencyEnabled(n, spec) {
			continue
		}
		status := s.states[spec.Rail]
		ready := s.fingerprint == fingerprint(c) && status.State == "ready" && status.UpdatedAt != nil && time.Since(*status.UpdatedAt) < 2*time.Minute
		result = append(result, model.PaymentMethod{ID: Provider + ":" + spec.Rail, Provider: Provider, ProviderName: c.Name, Rail: spec.Rail, Name: spec.Currency() + " " + spec.Name, Currency: spec.Currency(), CryptoCurrency: spec.Currency(), Network: spec.Network, NetworkName: spec.Name, Available: ready, Mode: "order"})
	}
	return result, nil
}

// Create recovers an issued quote before consulting mutable configuration.
func (s *Service) Create(ctx context.Context, orderID, rail, base string) (Quote, error) {
	if q, err := s.repository.USDTQuote(ctx, orderID); err == nil {
		return q, nil
	} else if !errors.Is(err, ErrQuoteNotFound) {
		return Quote{}, err
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	methods, err := s.Methods(ctx)
	if q, err := s.repository.USDTQuote(ctx, orderID); err == nil {
		return q, nil
	} else if !errors.Is(err, ErrQuoteNotFound) {
		return Quote{}, err
	}
	if err != nil {
		return Quote{}, err
	}
	enabled := false
	for _, m := range methods {
		if m.Rail == rail && m.Available {
			enabled = true
		}
	}
	if !enabled {
		return Quote{}, ErrUnavailable
	}
	c, err := s.config(ctx)
	if err != nil {
		return Quote{}, err
	}
	spec, ok := Network(rail)
	if !ok {
		return Quote{}, ErrConfiguration
	}
	for _, n := range c.Networks {
		if n.Network == spec.Network {
			return s.repository.ReserveUSDTQuote(ctx, orderID, spec, n.Addresses, base)
		}
	}
	return Quote{}, ErrUnavailable
}
