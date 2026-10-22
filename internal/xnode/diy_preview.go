package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func (s *Service) PreviewOwnedDIY(ctx context.Context, user string, input DIYWrite) (DIYPreview, error) {
	proxies, err := s.diyProxies(ctx, user)
	if err != nil {
		return DIYPreview{}, err
	}
	input.Overrides = subscription.ActiveTuningFor(input.Overrides, proxies)
	result := DIYPreview{Billing: DIYBillingQuote{Charge: model.TXBMoney(0)}, FieldErrors: subscription.ValidateTuning(input.Overrides), Paths: []routingmatch.Path{}}
	if input.Whitelist != nil {
		_, current, err := s.readDIYSettings(ctx, user)
		if err != nil {
			return result, err
		}
		quota, err := s.whitelistQuota(ctx, user)
		if err != nil {
			return result, err
		}
		if _, err = normalizeWhitelist(current.Whitelist, *input.Whitelist, quota); err != nil {
			result.FieldErrors["whitelist"] = "whitelistInvalid"
		}
	}
	if input.SettingsRevision < 0 || input.RoutingRevision < 0 || input.SettingsRevision >= 9007199254740991 {
		result.FieldErrors["revision"] = "invalid"
	}
	if input.SplitProfiles != nil && validateSplitProfiles(*input.SplitProfiles, proxies) != nil {
		result.FieldErrors["splitProfiles"] = "splitInvalid"
	}
	previous, next, settingsErr := s.diySettingsForWrite(ctx, user, input, proxies)
	if settingsErr != nil && !errors.Is(settingsErr, ErrInvalid) {
		return result, settingsErr
	}
	if settingsErr == nil {
		routing, err := s.diyRouting(ctx, user)
		if err != nil {
			return result, err
		}
		result.Billing, err = s.diyBillingQuote(ctx, user)
		if err != nil {
			return result, err
		}
		result.Billing.Changed = !sameDIYValue(previous, next) || !sameDIYValue(routing.Plan.Choices, input.Choices)
		if !result.Billing.Changed {
			result.Billing.Charge = model.TXBMoney(0)
			result.Billing.UseQuota = false
			result.Billing.Available = true
		}
	}
	for i := range proxies {
		proxies[i].Tuning = &input.Overrides
	}
	if settingsErr == nil {
		proxies = appendSplitProxies(next.SplitProfiles, proxies)
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
