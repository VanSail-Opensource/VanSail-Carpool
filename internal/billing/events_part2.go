package billing

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) loadNewRate(ctx context.Context, provider string) (Decimal, error) {
	if provider == "builtin_usdt" {
		if s.builtin == nil {
			return Decimal{}, errRateNotConfigured
		}
		raw, err := s.builtin.Rate(ctx)
		if err != nil {
			return Decimal{}, err
		}
		rate, err := ParseDecimal(raw)
		if err != nil || !rate.Positive() {
			return Decimal{}, errRateNotConfigured
		}
		return rate, nil
	}
	rateRaw, err := s.settings.Plaintext(ctx, "billing.rate.txb_per_"+currencyCode(provider))
	if err != nil {
		return Decimal{}, fmt.Errorf("%w: %v", errRateNotConfigured, err)
	}
	rate, err := ParseDecimal(rateRaw)
	if err != nil || !rate.Positive() {
		return Decimal{}, errRateNotConfigured
	}
	return rate, nil
}

func currencyCode(provider string) string {
	switch provider {
	case "ezpay":
		return "cny"
	case "bepusdt":
		return "usd"
	case "builtin_usdt":
		return "usdt"
	default:
		return "xtr"
	}
}

func (s *Service) absolute(path string) string {
	result := *s.publicURL
	result.Path = strings.TrimRight(result.Path, "/") + path
	result.RawQuery = ""
	if split := strings.IndexByte(path, '?'); split >= 0 {
		result.Path = strings.TrimRight(s.publicURL.Path, "/") + path[:split]
		result.RawQuery = path[split+1:]
	}
	return result.String()
}
