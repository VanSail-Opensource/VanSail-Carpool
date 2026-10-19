package xnode

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

type SubscriptionFormatDiagnostic struct {
	Code              string `json:"code"`
	ExcludedEndpoints int    `json:"excludedEndpoints"`
}

func (s *Service) projectSubscriptionFormats(ctx context.Context, owner string, proxies []subscription.Proxy, result *SubscriptionMetadata) error {
	config, err := s.SubscriptionTemplates(ctx)
	if err != nil {
		return err
	}
	result.Formats = map[string]bool{}
	result.FormatDiagnostics = map[string]SubscriptionFormatDiagnostic{}
	for _, format := range []string{"base64", "native", "xray", "sing-box", "clash", "stash"} {
		_, excluded, _ := subscription.CompatibleProxies(format, proxies)
		_, err := renderSubscriptionSelection(config, owner, SubscriptionSelection{Format: format}, proxies)
		result.Formats[format] = err == nil && result.Active
		code := ""
		if err != nil && config.Formats[format] && result.Active {
			code = "SUBSCRIPTION_FORMAT_RENDER_FAILED"
			var policy *PolicyError
			if errors.As(err, &policy) {
				code = policy.Code
			}
		} else if excluded > 0 && config.Formats[format] && result.Active {
			code = "SUBSCRIPTION_ENDPOINTS_EXCLUDED"
		}
		if code != "" {
			result.FormatDiagnostics[format] = SubscriptionFormatDiagnostic{Code: code, ExcludedEndpoints: excluded}
		}
	}
	return nil
}
