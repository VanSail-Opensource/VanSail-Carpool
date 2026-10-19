package usdt

import "slices"

// A missing list inherits the legacy mode. An explicit empty list accepts no new
// quotes; monitor still follows durable outstanding quotes independently.
func normalizeCurrencies(n *NetworkConfig, mode string) error {
	registered := []string{}
	for _, spec := range Registry {
		if spec.Network == n.Network {
			registered = append(registered, spec.Currency())
		}
	}
	if n.Currencies == nil {
		n.Currencies = []string{}
		for _, spec := range Registry {
			if spec.Network == n.Network && modeAllows(mode, spec) {
				n.Currencies = append(n.Currencies, spec.Currency())
			}
		}
		return nil
	}
	if len(n.Currencies) > len(registered) {
		return ErrConfiguration
	}
	seen := map[string]bool{}
	for _, currency := range n.Currencies {
		if !slices.Contains(registered, currency) || seen[currency] {
			return ErrConfiguration
		}
		seen[currency] = true
	}
	// Canonical registry order prevents a purely cosmetic readiness restart.
	n.Currencies = []string{}
	for _, currency := range registered {
		if seen[currency] {
			n.Currencies = append(n.Currencies, currency)
		}
	}
	return nil
}

func currencyEnabled(n NetworkConfig, spec NetworkSpec) bool {
	return n.Network == spec.Network && slices.Contains(n.Currencies, spec.Currency())
}
