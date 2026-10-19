package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func (s *Service) renderOwnedSubscription(ctx context.Context, user, format string, proxies []subscription.Proxy) ([]byte, error) {
	return s.renderSelectedSubscription(ctx, user, SubscriptionSelection{Format: format}, proxies)
}

func (s *Service) renderSelectedSubscription(ctx context.Context, user string, selection SubscriptionSelection, proxies []subscription.Proxy) ([]byte, error) {
	config, err := s.SubscriptionTemplates(ctx)
	if err != nil {
		return nil, err
	}
	return renderSubscriptionSelection(config, user, selection, proxies)
}

func renderSubscriptionSelection(config SubscriptionTemplateConfig, user string, selection SubscriptionSelection, proxies []subscription.Proxy) ([]byte, error) {
	format := selection.Format
	if !config.Formats[format] {
		return nil, &PolicyError{"SUBSCRIPTION_FORMAT_DISABLED", ErrInvalid}
	}
	if format == "native" || format == "base64" {
		raw, err := renderSubscription(format, proxies)
		if err != nil {
			return nil, subscriptionRenderError(err)
		}
		return raw, nil
	}
	selected, _, err := subscription.CompatibleProxies(format, proxies)
	if err != nil {
		return nil, subscriptionRenderError(err)
	}
	id := config.Defaults[format]
	if override, exists := config.Assignments[user][format]; exists {
		id = override
	}
	if selection.TemplateID != "" {
		id = selection.TemplateID
	}
	item := subscription.DefaultTemplate(format)
	if id != "" {
		found := false
		for _, candidate := range config.Templates {
			if candidate.ID == id && candidate.Format == format {
				item = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, &PolicyError{"SUBSCRIPTION_FORMAT_RENDER_FAILED", ErrInvalid}
		}
	}
	raw, err := subscription.RenderTemplate(item, selected)
	if err != nil {
		return nil, subscriptionRenderError(err)
	}
	return raw, nil
}

func subscriptionRenderError(err error) error {
	code := "SUBSCRIPTION_FORMAT_RENDER_FAILED"
	if errors.Is(err, subscription.ErrNoCompatibleProxies) {
		code = "SUBSCRIPTION_FORMAT_NO_ENDPOINTS"
	}
	return &PolicyError{code, errors.Join(ErrInvalid, err)}
}
