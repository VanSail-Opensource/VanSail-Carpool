package xnode

import (
	"context"
	"crypto/sha256"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"slices"
	"strconv"
	"time"
)

type SubscriptionMetadata struct {
	Username   string          `json:"username"`
	URL        string          `json:"url"`
	UsedBytes  string          `json:"usedBytes"`
	LimitBytes string          `json:"limitBytes"`
	ExpiresAt  *time.Time      `json:"expiresAt"`
	Active     bool            `json:"active"`
	Incomplete bool            `json:"incomplete"`
	Formats    map[string]bool `json:"formats"`
}

func (s *Service) bearerIdentity(ctx context.Context, token string) (Identity, error) {
	if len(token) != 43 {
		return Identity{}, ErrNotFound
	}
	hash := sha256.Sum256([]byte(token))
	identity, err := s.repo.XNodeIdentityBySubscription(ctx, hash[:])
	if err != nil || identity.UserID == "" {
		return Identity{}, ErrNotFound
	}
	return identity, nil
}
func (s *Service) BearerMetadata(ctx context.Context, token string) (SubscriptionMetadata, error) {
	identity, err := s.bearerIdentity(ctx, token)
	if err != nil {
		return SubscriptionMetadata{}, err
	}
	return s.subscriptionMetadata(ctx, identity)
}
func (s *Service) OwnedSubscription(ctx context.Context, user string) (SubscriptionMetadata, error) {
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return SubscriptionMetadata{}, err
	}
	return s.subscriptionMetadata(ctx, identity)
}
func (s *Service) subscriptionMetadata(ctx context.Context, identity Identity) (SubscriptionMetadata, error) {
	result := SubscriptionMetadata{Username: identity.Username, UsedBytes: "0", LimitBytes: "0", Formats: map[string]bool{}}
	result.URL, _ = s.SubscriptionURL(identity)
	purchase, err := s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return result, err
	}
	if purchase != nil {
		until := purchase.ValidUntil
		result.ExpiresAt = &until
		result.LimitBytes = strconv.FormatInt(purchase.TrafficLimitBytes, 10)
	}
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return result, err
		}
		result.UsedBytes = strconv.FormatInt(period.UsedBytes, 10)
	}
	proxies, incomplete, err := s.memberInventory(ctx, identity.UserID)
	if err != nil {
		return result, err
	}
	result.Active = len(proxies) > 0
	result.Incomplete = incomplete
	for _, format := range []string{"native", "xray", "sing-box", "clash"} {
		_, err := renderSubscription(format, proxies)
		result.Formats[format] = err == nil && result.Active
	}
	return result, nil
}
func (s *Service) BearerSubscription(ctx context.Context, token, format string) ([]byte, SubscriptionMetadata, error) {
	identity, err := s.bearerIdentity(ctx, token)
	if err != nil {
		return nil, SubscriptionMetadata{}, err
	}
	metadata, err := s.subscriptionMetadata(ctx, identity)
	if err != nil {
		return nil, metadata, err
	}
	if !metadata.Active {
		return nil, metadata, ErrNotFound
	}
	if !metadata.Formats[format] {
		return nil, metadata, ErrInvalid
	}
	proxies, err := s.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return nil, metadata, err
	}
	raw, err := renderSubscription(format, proxies)
	return raw, metadata, err
}
func renderSubscription(format string, proxies []subscription.Proxy) ([]byte, error) {
	sourceFormat := map[string]string{"native": "XRAY_BASE64", "xray": "XRAY_JSON", "sing-box": "SINGBOX", "clash": "MIHOMO"}[format]
	if sourceFormat == "" {
		return nil, ErrInvalid
	}
	selected := make([]subscription.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if !slices.Contains(proxy.ExcludedFormats, sourceFormat) {
			selected = append(selected, proxy)
		}
	}
	if len(selected) == 0 {
		return nil, ErrNotFound
	}
	proxies = selected
	switch format {
	case "native":
		return subscription.RenderNative(proxies)
	case "xray":
		return subscription.RenderXray(proxies)
	case "sing-box":
		return subscription.RenderSingBox(proxies)
	case "clash":
		return subscription.RenderClash(proxies)
	default:
		return nil, ErrInvalid
	}
}
func (s *Service) RoutingTemplates(ctx context.Context, user string) ([]map[string]any, error) {
	proxies, err := s.MemberProxies(ctx, user)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, proxy := range proxies {
		value, err := proxy.Outbound()
		if err != nil {
			return nil, err
		}
		suffix := proxy.ID
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		value["tag"] = "peer-" + suffix
		out = append(out, map[string]any{"nodeId": proxy.NodeID, "name": proxy.Name, "outbound": value})
	}
	return out, nil
}
