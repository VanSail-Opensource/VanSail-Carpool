package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func (s *Service) readDIY(ctx context.Context, user string) (DIYDocument, subscription.ConnectionOverrides, error) {
	doc, settings, err := s.readDIYSettings(ctx, user)
	return doc, settings.Overrides, err
}

func (s *Service) diyRouting(ctx context.Context, user string) (RoutingView, error) {
	view, err := s.OwnedRoutingPlan(ctx, user)
	if !isMissing(err) {
		return view, err
	}
	catalog, err := s.RoutingTemplateConfig(ctx)
	return RoutingView{Plan: RoutingPlan{Phase: "active", Choices: []RoutingChoice{}}, Templates: catalog.Templates, Nodes: []RoutingNode{}, Paths: view.Paths, Endpoints: []RoutingEndpoint{}}, err
}

func (s *Service) diyProxies(ctx context.Context, user string) ([]subscription.Proxy, error) {
	if _, err := s.repo.XNodeIdentityByUser(ctx, user); isMissing(err) {
		return []subscription.Proxy{}, nil
	} else if err != nil {
		return nil, err
	}
	proxies, _, err := s.memberInventory(ctx, user)
	return proxies, err
}

func (s *Service) OwnedDIY(ctx context.Context, user string) (DIYView, error) {
	view := DIYView{}
	pending, err := s.requestDIYPAYG(ctx, user)
	if err != nil {
		return view, err
	}
	view.InventoryPending = pending
	doc, settings, err := s.readDIYSettings(ctx, user)
	if err != nil {
		return view, err
	}
	view.SettingsRevision, view.Overrides = doc.Revision, settings.Overrides
	view.Whitelist, err = s.ownedWhitelist(ctx, user, settings.Whitelist)
	if err != nil {
		return view, err
	}
	view.Routing, err = s.diyRouting(ctx, user)
	if err != nil {
		return view, err
	}
	proxies, err := s.diyProxies(ctx, user)
	if err != nil {
		return view, err
	}
	view.Fields = subscription.TuningFieldsFor(proxies)
	view.SplitProfiles = validSplitProfiles(settings.SplitProfiles, proxies)
	view.SplitHosts = splitHostCatalog(proxies)
	view.Billing, err = s.diyBillingQuote(ctx, user)
	if err != nil {
		return view, err
	}
	view.Compatibility = subscription.TuningCompatibility(settings.Overrides, proxies)
	return view, nil
}

// ExportProxiesForUser annotates only owner exports/host links. Routing,
// transit peers, monitoring identities and connectivity probes use untuned data.
func (s *Service) ExportProxiesForUser(ctx context.Context, user string, proxies []subscription.Proxy) ([]subscription.Proxy, error) {
	if user == "" {
		return proxies, nil
	}
	if _, ok := s.repo.(diyRepository); !ok {
		return proxies, nil
	}
	_, settings, err := s.readDIYSettings(ctx, user)
	if err != nil {
		return nil, err
	}
	overrides := subscription.ActiveTuningFor(settings.Overrides, proxies)
	if len(subscription.ValidateTuning(overrides)) > 0 {
		return nil, ErrInvalid
	}
	result := append([]subscription.Proxy(nil), proxies...)
	for index := range result {
		result[index].Tuning = &overrides
	}
	return appendSplitProxies(settings.SplitProfiles, result), nil
}
