package xnode

import (
	"context"
	"encoding/json"
)

// DDNSHostBinding stores panel-owned references separately from agent discovery.
// ConfiguredBy is authoritative audit ownership and never accepted from JSON.
type DDNSHostBinding struct {
	IPv4HostIDs  []string `json:"ipv4HostIds,omitempty"`
	IPv6HostIDs  []string `json:"ipv6HostIds,omitempty"`
	ConfiguredBy string   `json:"-"`
}

// UnmarshalJSON accepts the former single-host fields during rolling upgrades.
func (b *DDNSHostBinding) UnmarshalJSON(data []byte) error {
	var input struct {
		IPv4HostIDs []string `json:"ipv4HostIds"`
		IPv6HostIDs []string `json:"ipv6HostIds"`
		IPv4HostID  string   `json:"ipv4HostId"`
		IPv6HostID  string   `json:"ipv6HostId"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}
	b.IPv4HostIDs = append([]string(nil), input.IPv4HostIDs...)
	b.IPv6HostIDs = append([]string(nil), input.IPv6HostIDs...)
	if len(b.IPv4HostIDs) == 0 && input.IPv4HostID != "" {
		b.IPv4HostIDs = []string{input.IPv4HostID}
	}
	if len(b.IPv6HostIDs) == 0 && input.IPv6HostID != "" {
		b.IPv6HostIDs = []string{input.IPv6HostID}
	}
	return nil
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
