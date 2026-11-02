package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// Export restrictions affect fresh client documents, never native authorization,
// routing dependencies or an already connected principal. Filter before split
// profiles are derived so omitted credentials cannot reappear in combined hosts.
func (s *Service) visiblePAYGExports(ctx context.Context, user string, proxies []subscription.Proxy) ([]subscription.Proxy, error) {
	repo, ok := s.paygRepository()
	if !ok {
		return proxies, nil
	}
	// Combo proxies have no internal PAYG principal, even when they share an
	// endpoint or group with PAYG. Avoid a balance read for combo-only exports.
	hasPAYG := false
	for _, proxy := range proxies {
		if proxy.PrincipalID != 0 {
			hasPAYG = true
			break
		}
	}
	if !hasPAYG {
		return proxies, nil
	}
	balance, err := repo.Balance(ctx, user)
	if err != nil {
		return nil, err
	}
	products, err := repo.ListSquadProductOverrides(ctx, false)
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
		omit := proxy.PrincipalID != 0 && balance.MinorInt64() <= 0
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
