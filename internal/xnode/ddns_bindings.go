package xnode

import "context"

// DDNSHostBinding stores panel-owned references separately from agent discovery.
// ConfiguredBy is authoritative audit ownership and never accepted from JSON.
type DDNSHostBinding struct {
	IPv4HostID   string `json:"ipv4HostId,omitempty"`
	IPv6HostID   string `json:"ipv6HostId,omitempty"`
	ConfiguredBy string `json:"-"`
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
	user, err := s.repo.UserByID(ctx, actor)
	if err != nil {
		return node, err
	}
	if user.Role != "admin" {
		return node, ErrInvalid
	}
	settings.ddnsActor = actor
	return s.SaveNode(ctx, node, settings)
}

func (s *Service) persistNodeDDNS(ctx context.Context, node Node, bindings map[string]DDNSHostBinding) error {
	if target, ok := s.repo.(ddnsBindingWriter); ok {
		return target.SaveXNodeNodeDDNS(ctx, node, bindings)
	}
	if len(bindings) > 0 {
		return ErrUnavailable
	}
	return s.repo.SaveXNodeNode(ctx, node)
}
