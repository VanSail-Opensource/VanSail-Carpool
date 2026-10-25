package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// Hiding affects fresh client documents, never native authorization, routing
// dependencies or an already connected principal. Filter before split profiles
// are derived so hidden credentials cannot reappear through a combined host.
func (s *Service) visiblePAYGExports(ctx context.Context, proxies []subscription.Proxy) ([]subscription.Proxy, error) {
	repo, ok := s.paygRepository()
	if !ok {
		return proxies, nil
	}
	products, err := repo.ListSquadProducts(ctx, false)
	if err != nil {
		return nil, err
	}
	hidden := map[string]bool{}
	for _, product := range products {
		if product.BillingMode == "payg" && product.PAYGState == "hidden" {
			hidden[product.GroupID] = true
		}
	}
	visible := make([]subscription.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		omit := false
		if proxy.PrincipalID != 0 {
			for _, group := range proxy.GroupIDs {
				omit = omit || hidden[group]
			}
		}
		if !omit {
			visible = append(visible, proxy)
		}
	}
	return visible, nil
}
