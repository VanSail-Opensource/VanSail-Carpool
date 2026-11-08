package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
)

// Renewal prices follow the public product price within the purchased contract
// bounds. Individual behavior, costs and coupon ownership never choose a price.
func (s *Store) calculateRenewalPrice(_ context.Context, in renewalPriceSnapshot) (renewal.Candidate, []renewal.Candidate, error) {
	c := in.Contract
	price := c.Price
	if in.PublicEnabled {
		limit := min(c.Ceiling, c.Price+c.Price*c.MaxChangeBPS/10000)
		price = min(in.Public, limit)
		var addonBasis int64
		for _, addon := range c.Addons {
			addonBasis += addon.Price
		}
		price = max(price, addonBasis)
	}
	candidate := renewal.Candidate{Price: price, Eligible: true}
	return candidate, []renewal.Candidate{candidate}, nil
}
