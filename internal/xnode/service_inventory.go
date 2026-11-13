package xnode

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) Groups(ctx context.Context) ([]Group, error) { return s.repo.XNodeGroups(ctx) }
func (s *Service) SaveGroup(ctx context.Context, group Group) (Group, error) {
	return s.saveGroup(ctx, group, "")
}

func (s *Service) saveGroup(ctx context.Context, group Group, actor string) (Group, error) {
	if strings.TrimSpace(group.Name) == "" || len(group.Name) > 128 {
		return group, ErrInvalid
	}
	if group.ID == "" {
		var err error
		group.ID, err = ids.New()
		if err != nil {
			return group, err
		}
	}
	seen := map[string]bool{}
	for _, binding := range group.Bindings {
		if !wire.ValidName(binding.InboundTag) {
			return group, ErrInvalid
		}
		if err := s.validateManagedBinding(ctx, binding.NodeID, binding.InboundTag); err != nil {
			return group, err
		}
		key := binding.NodeID + ":" + binding.InboundTag
		if seen[key] {
			return group, ErrInvalid
		}
		seen[key] = true
	}
	return s.persistGroup(ctx, actor, group)
}
func (s *Service) Endpoints(ctx context.Context) ([]Endpoint, error) {
	items, err := s.repo.XNodeEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		raw, err := s.vault.Decrypt("xnode.endpoint."+items[i].ID, items[i].OverridesCipher)
		if err != nil {
			return nil, err
		}
		items[i].Overrides = json.RawMessage(raw)
	}
	return items, nil
}
func (s *Service) SaveEndpoint(ctx context.Context, item Endpoint) (Endpoint, error) {
	return s.saveEndpoint(ctx, item, "")
}

func (s *Service) saveEndpoint(ctx context.Context, item Endpoint, actor string) (Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, err := s.PreviewEndpoint(ctx, item)
	if err != nil {
		return item, err
	}
	if !preview.Valid {
		return item, ErrInvalid
	}
	item.Metadata = &preview.Metadata
	if item.ID == "" {
		var err error
		item.ID, err = ids.New()
		if err != nil {
			return item, err
		}
	}
	if len(item.Overrides) == 0 {
		item.Overrides = json.RawMessage(`{}`)
	}
	var overrides map[string]any
	if json.Unmarshal(item.Overrides, &overrides) != nil {
		return item, ErrInvalid
	}
	if _, err := filterEndpointGroups(nil, item.Overrides); err != nil {
		return item, err
	}
	item.OverridesCipher, err = s.vault.Encrypt("xnode.endpoint."+item.ID, string(item.Overrides))
	if err != nil {
		return item, err
	}
	return s.persistEndpoint(ctx, actor, item)
}
