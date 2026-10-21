package xnode

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func (s *Service) PreviewOwnedDIY(ctx context.Context, user string, input DIYWrite) (DIYPreview, error) {
	result := DIYPreview{FieldErrors: subscription.ValidateTuning(input.Overrides), Paths: []routingmatch.Path{}}
	if input.SettingsRevision < 0 || input.RoutingRevision < 0 || input.SettingsRevision >= 9007199254740991 {
		result.FieldErrors["revision"] = "invalid"
	}
	proxies, err := s.diyProxies(ctx, user)
	if err != nil {
		return result, err
	}
	result.Compatibility = subscription.TuningCompatibility(input.Overrides, proxies)
	if len(input.Choices) > 0 {
		paths, _, err := s.PreviewOwnedRouting(ctx, user, input.Choices, "", routingmatch.Destination{})
		if err != nil {
			var diagnostic *routingmatch.Diagnostic
			if errors.As(err, &diagnostic) {
				result.FieldErrors["routing"] = diagnostic.Code
			} else {
				result.FieldErrors["routing"] = "routing_endpoint_unavailable"
			}
		} else {
			result.Paths = paths
		}
	}
	if len(result.FieldErrors) == 0 {
		eligible, _, err := subscription.CompatibleProxies("xray", proxies)
		if err == nil {
			for _, proxy := range eligible {
				proxy.Tuning = &input.Overrides
				out, err := proxy.Outbound()
				if err != nil {
					result.FieldErrors["overrides"] = "xrayValidation"
					break
				}
				raw, err := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": []any{out}})
				if err != nil {
					return result, err
				}
				if validation := ValidateClientConfigurationDocument(raw); !validation.Valid {
					result.FieldErrors["overrides"] = "xrayValidation"
					break
				}
			}
		}
	}
	result.Valid = len(result.FieldErrors) == 0
	return result, nil
}
