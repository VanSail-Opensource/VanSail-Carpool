package xnode

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func (s *Service) readDIY(ctx context.Context, user string) (DIYDocument, subscription.ConnectionOverrides, error) {
	repo, ok := s.repo.(diyRepository)
	if !ok {
		return DIYDocument{}, subscription.ConnectionOverrides{}, ErrUnavailable
	}
	doc, err := repo.XNodeDIY(ctx, user)
	if err != nil {
		return doc, subscription.ConnectionOverrides{}, err
	}
	var overrides subscription.ConnectionOverrides
	if doc.Cipher != "" {
		if err = s.open("xnode.diy."+user, doc.Cipher, &overrides); err != nil {
			return doc, overrides, err
		}
	}
	return doc, overrides, nil
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
	view := DIYView{Fields: subscription.TuningFields()}
	doc, overrides, err := s.readDIY(ctx, user)
	if err != nil {
		return view, err
	}
	view.SettingsRevision, view.Overrides = doc.Revision, overrides
	view.Routing, err = s.diyRouting(ctx, user)
	if err != nil {
		return view, err
	}
	proxies, err := s.diyProxies(ctx, user)
	if err != nil {
		return view, err
	}
	view.Compatibility = subscription.TuningCompatibility(overrides, proxies)
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
	_, overrides, err := s.readDIY(ctx, user)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(overrides)
	if err != nil {
		return nil, err
	}
	if string(raw) == "{}" {
		return proxies, nil
	}
	if len(subscription.ValidateTuning(overrides)) > 0 {
		return nil, fmt.Errorf("%w: stored client options", ErrInvalid)
	}
	result := append([]subscription.Proxy(nil), proxies...)
	for index := range result {
		result[index].Tuning = &overrides
	}
	return result, nil
}
