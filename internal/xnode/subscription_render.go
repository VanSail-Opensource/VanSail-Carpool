package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"slices"
)

func (s *Service) renderOwnedSubscription(ctx context.Context, user, format string, proxies []subscription.Proxy) ([]byte, error) {
	return s.renderSelectedSubscription(ctx, user, SubscriptionSelection{Format: format}, proxies)
}

func (s *Service) renderSelectedSubscription(ctx context.Context, user string, selection SubscriptionSelection, proxies []subscription.Proxy) ([]byte, error) {
	format := selection.Format
	config, err := s.SubscriptionTemplates(ctx)
	if err != nil {
		return nil, err
	}
	if !config.Formats[format] {
		return nil, ErrInvalid
	}
	if format == "native" || format == "base64" {
		return renderSubscription(format, proxies)
	}
	selected := []subscription.Proxy{}
	sourceFormat := map[string]string{"xray": "XRAY_JSON", "sing-box": "SINGBOX", "clash": "MIHOMO", "stash": "STASH"}[format]
	if sourceFormat == "" {
		return nil, ErrInvalid
	}
	for _, proxy := range proxies {
		if !slices.Contains(proxy.ExcludedFormats, sourceFormat) {
			selected = append(selected, proxy)
		}
	}
	if len(selected) == 0 {
		return nil, ErrNotFound
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
			return nil, ErrInvalid
		}
	}
	return subscription.RenderTemplate(item, selected)
}
