package xnode

import (
	"context"
	"crypto/sha256"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SubscriptionMetadata struct {
	SelectedFormat     string                                  `json:"selectedFormat,omitempty"`
	SelectedTemplateID string                                  `json:"selectedTemplateId,omitempty"`
	SelectionSource    string                                  `json:"selectionSource,omitempty"`
	Username           string                                  `json:"username"`
	URL                string                                  `json:"url"`
	DownloadURL        string                                  `json:"downloadUrl"`
	UsedBytes          string                                  `json:"usedBytes"`
	LimitBytes         string                                  `json:"limitBytes"`
	ExpiresAt          *time.Time                              `json:"expiresAt"`
	Active             bool                                    `json:"active"`
	Incomplete         bool                                    `json:"incomplete"`
	Formats            map[string]bool                         `json:"formats"`
	FormatDiagnostics  map[string]SubscriptionFormatDiagnostic `json:"formatDiagnostics,omitempty"`
}

func (s *Service) bearerIdentity(ctx context.Context, token string) (Identity, error) {
	if len(token) != 43 {
		return Identity{}, ErrNotFound
	}
	hash := sha256.Sum256([]byte(token))
	identity, err := s.repo.XNodeIdentityBySubscription(ctx, hash[:])
	if err != nil {
		return Identity{}, ErrNotFound
	}
	if identity.UserID == "" {
		access, err := s.ServiceAccess(ctx, identity)
		if err != nil || access == nil {
			return Identity{}, ErrNotFound
		}
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
	if identity.UserID == "" {
		return s.serviceSubscriptionMetadata(ctx, identity)
	}
	result := SubscriptionMetadata{Username: identity.Username, UsedBytes: "0", LimitBytes: "0", Formats: map[string]bool{}}
	var err error
	result.URL, err = s.SubscriptionURL(identity)
	if err != nil {
		return result, err
	}
	download, err := url.Parse(result.URL)
	if err != nil {
		return result, ErrInvalid
	}
	download.Path = "/api/v1/subscriptions/" + strings.TrimPrefix(download.Path, "/subscription/")
	download.RawPath = ""
	result.DownloadURL = download.String()
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
	if err := s.projectSubscriptionFormats(ctx, identity.UserID, proxies, &result); err != nil {
		return result, err
	}
	return result, nil
}
func (s *Service) BearerSubscription(ctx context.Context, token, format string) ([]byte, SubscriptionMetadata, error) {
	return s.BearerSubscriptionForAgent(ctx, token, format, "")
}
func (s *Service) BearerSubscriptionForAgent(ctx context.Context, token, explicit, agent string) ([]byte, SubscriptionMetadata, error) {
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
	config, err := s.SubscriptionTemplates(ctx)
	if err != nil {
		return nil, metadata, err
	}
	selection, err := SelectSubscription(config, explicit, agent)
	if err != nil {
		return nil, metadata, err
	}
	if !config.Formats[selection.Format] {
		return nil, metadata, ErrInvalid
	}
	proxies, _, err := s.identityInventory(ctx, identity)
	if err != nil {
		return nil, metadata, err
	}
	owner := identity.UserID
	if owner == "" {
		owner = identityScope(identity)
	}
	metadata.SelectedFormat, metadata.SelectedTemplateID, metadata.SelectionSource = selection.Format, selection.TemplateID, selection.Source
	raw, err := renderSubscriptionSelection(config, owner, selection, proxies)
	return raw, metadata, err
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
