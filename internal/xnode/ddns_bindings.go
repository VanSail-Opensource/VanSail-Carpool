package xnode

import (
	"context"
)

// DDNSHostBinding stores panel-owned references separately from agent discovery.
// ConfiguredBy is authoritative audit ownership and never accepted from JSON.
type DDNSHostBinding struct {
	IPv4HostIDs  []string `json:"ipv4HostIds,omitempty"`
	IPv6HostIDs  []string `json:"ipv6HostIds,omitempty"`
	ConfiguredBy string   `json:"-"`
}

type ddnsBindingReader interface {
	XNodeDDNSBindings(context.Context, string) (map[string]DDNSHostBinding, error)
}
type ddnsBindingWriter interface {
	SaveXNodeNodeDDNS(context.Context, Node, map[string]DDNSHostBinding) error
}

func (s *Service) loadDDNSBindings(ctx context.Context, node string) (map[string]DDNSHostBinding, error) {
	if source, ok := s.repo.(ddnsBindingReader); ok {
		return source.XNodeDDNSBindings(ctx, node)
	}
	return map[string]DDNSHostBinding{}, nil
}

// SaveNodeAs attributes future automatic node operations to the administrator
// who configured them; there is no invented system member or financial owner.
func (s *Service) SaveNodeAs(ctx context.Context, node Node, settings NodeSettings, actor string) (Node, error) {
	if actor == "" {
		return node, ErrInvalid
	}
	settings.ddnsActor = actor
	return s.saveNode(ctx, node, settings, actor)
}

func (s *Service) persistNodeDDNS(ctx context.Context, node Node, bindings map[string]DDNSHostBinding, actor string) (Node, error) {
	if actor != "" {
		target, ok := s.repo.(interface {
			SaveAdminXNodeNodeDDNS(context.Context, string, Node, map[string]DDNSHostBinding) (Node, error)
		})
		if !ok {
			return node, ErrUnavailable
		}
		return target.SaveAdminXNodeNodeDDNS(ctx, actor, node, bindings)
	}
	if target, ok := s.repo.(ddnsBindingWriter); ok {
		if err := target.SaveXNodeNodeDDNS(ctx, node, bindings); err != nil {
			return node, err
		}
		return s.repo.XNodeNode(ctx, node.ID)
	}
	if len(bindings) > 0 {
		return node, ErrUnavailable
	}
	if err := s.repo.SaveXNodeNode(ctx, node); err != nil {
		return node, err
	}
	return s.repo.XNodeNode(ctx, node.ID)
}
