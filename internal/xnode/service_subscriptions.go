package xnode

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// ServiceSubscription exposes a standalone account's bearer only to its caller;
// the HTTP entry point is administrator-only and uses private response headers.
func (s *Service) ServiceSubscription(ctx context.Context, id string) (SubscriptionMetadata, error) {
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return SubscriptionMetadata{}, err
	}
	return s.serviceSubscriptionMetadata(ctx, identity)
}

func (s *Service) identityInventory(ctx context.Context, identity Identity) ([]subscription.Proxy, bool, error) {
	if identity.UserID != "" {
		return s.memberInventory(ctx, identity.UserID)
	}
	access, err := s.ServiceAccess(ctx, identity)
	if err != nil || access == nil {
		return nil, false, ErrNotFound
	}
	period, err := s.servicePeriod(ctx, identity, access)
	if err != nil || period == nil {
		return []subscription.Proxy{}, false, err
	}
	out := []subscription.Proxy{}
	var legacyError error
	if s.legacyProxies != nil && identity.LegacyUserID != "" {
		out, legacyError = s.legacyProxies(ctx, identity)
		if legacyError != nil {
			out = []subscription.Proxy{}
		}
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return nil, false, err
	}
	groups, err := s.Groups(ctx)
	if err != nil {
		return nil, false, err
	}
	enabled := map[string]bool{}
	for _, group := range groups {
		enabled[group.ID] = group.Enabled
	}
	for _, endpoint := range endpoints {
		if !endpoint.Enabled {
			continue
		}
		node, err := s.repo.XNodeNode(ctx, endpoint.NodeID)
		if err != nil {
			return nil, false, err
		}
		owned, err := s.NativeOwner(ctx, node.ID)
		if err != nil {
			return nil, false, err
		}
		if !node.Enabled || !owned {
			continue
		}
		proxy, err := s.ServiceProxy(ctx, identity, endpoint)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		proxy.GroupIDs = slices.DeleteFunc(proxy.GroupIDs, func(id string) bool { return !enabled[id] || !slices.Contains(access.GroupIDs, id) })
		proxy.GroupIDs, err = filterEndpointGroups(proxy.GroupIDs, endpoint.Overrides)
		if err != nil {
			return nil, false, err
		}
		if len(proxy.GroupIDs) > 0 {
			out = append(out, proxy)
		}
	}
	if len(out) == 0 && legacyError != nil {
		return nil, true, ErrUnavailable
	}
	return out, legacyError != nil, nil
}

func (s *Service) serviceSubscriptionMetadata(ctx context.Context, identity Identity) (SubscriptionMetadata, error) {
	result := SubscriptionMetadata{Username: identity.ServiceName, UsedBytes: "0", LimitBytes: "0", Formats: map[string]bool{}}
	access, err := s.ServiceAccess(ctx, identity)
	if err != nil {
		return result, err
	}
	if access == nil {
		return result, ErrNotFound
	}
	result.URL, err = s.SubscriptionURL(identity)
	if err != nil {
		return result, err
	}
	download, err := url.Parse(result.URL)
	if err != nil {
		return result, err
	}
	download.Path = "/api/v1/subscriptions/" + strings.TrimPrefix(download.Path, "/subscription/")
	download.RawPath = ""
	result.DownloadURL = download.String()
	result.ExpiresAt, result.LimitBytes = &access.ExpiresAt, strconv.FormatInt(access.LimitBytes, 10)
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return result, err
		}
		result.UsedBytes, result.LimitBytes = strconv.FormatInt(period.UsedBytes, 10), strconv.FormatInt(period.LimitBytes, 10)
	}
	proxies, incomplete, err := s.identityInventory(ctx, identity)
	if err != nil {
		return result, err
	}
	result.Active, result.Incomplete = len(proxies) > 0, incomplete
	for _, format := range []string{"native", "xray", "sing-box", "clash"} {
		_, err := s.renderOwnedSubscription(ctx, identityScope(identity), format, proxies)
		result.Formats[format] = err == nil && result.Active
	}
	return result, nil
}
