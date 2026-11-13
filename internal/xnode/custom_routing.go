package xnode

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/validation"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) SaveOwnedOutbounds(ctx context.Context, user, node string, revision int64, raw json.RawMessage) (Customization, error) {
	return s.saveOwnedOutbounds(ctx, user, node, revision, raw, "")
}

func (s *Service) saveOwnedOutbounds(ctx context.Context, user, node string, revision int64, raw json.RawMessage, actor string) (Customization, error) {
	identity, tags, err := s.customizationIdentity(ctx, user, node)
	if err != nil {
		return Customization{}, err
	}
	if len(tags) == 0 {
		return Customization{}, ErrNotFound
	}
	item, err := s.customization(ctx, user, node)
	if err != nil {
		return item, err
	}
	if item.Revision != revision {
		return item, ErrConflict
	}
	if len(raw) > 64<<10 || validation.JSONDocument(raw) != nil {
		return item, ErrInvalid
	}
	var outbounds []map[string]any
	if wire.Decode(raw, &outbounds) != nil || len(outbounds) > 16 {
		return item, ErrInvalid
	}
	names := map[string]string{}
	for _, out := range outbounds {
		tag, _ := out["tag"].(string)
		name, err := namespaceTag(tag, identity.Username)
		if err != nil || names[tag] != "" {
			return item, ErrInvalid
		}
		names[tag] = name
	}
	for _, out := range outbounds {
		if err = s.validateMemberOutbound(ctx, out, names); err != nil {
			return item, err
		}
		tag, _ := out["tag"].(string)
		out["tag"] = names[tag]
		rewriteDependencies(out, func(tag string) string { return names[tag] })
	}
	// DNS validation runs on the existing queue without holding the service
	// mutation lock. Recheck owner, access and revision before using its result.
	s.mu.Lock()
	defer s.mu.Unlock()
	current, currentTags, err := s.customizationIdentity(ctx, user, node)
	if err != nil {
		return item, err
	}
	if len(currentTags) == 0 || current.ProxyUserID != identity.ProxyUserID || current.Username != identity.Username {
		return item, ErrConflict
	}
	item, err = s.customization(ctx, user, node)
	if err != nil {
		return item, err
	}
	if item.Revision != revision {
		return item, ErrConflict
	}
	var rules []map[string]any
	if item.RoutingCipher != "" {
		if err = s.open("xnode.routing."+user+"."+node, item.RoutingCipher, &rules); err != nil {
			return item, err
		}
	}
	saved := map[string]bool{}
	for _, name := range names {
		saved[name] = true
	}
	for _, rule := range rules {
		tag, _ := rule["outboundTag"].(string)
		if !saved[tag] {
			return item, ErrConflict
		}
	}
	if rules == nil {
		rules = []map[string]any{}
	}
	return s.storeCustomization(ctx, item, identity, outbounds, rules, actor)
}
func (s *Service) SaveOwnedRouting(ctx context.Context, user, node string, revision int64, raw json.RawMessage) (Customization, error) {
	return s.saveOwnedRouting(ctx, user, node, revision, raw, "")
}

func (s *Service) saveOwnedRouting(ctx context.Context, user, node string, revision int64, raw json.RawMessage, actor string) (Customization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, tags, err := s.customizationIdentity(ctx, user, node)
	if err != nil {
		return Customization{}, err
	}
	if len(tags) == 0 {
		return Customization{}, ErrNotFound
	}
	item, err := s.customization(ctx, user, node)
	if err != nil {
		return item, err
	}
	if item.Revision != revision {
		return item, ErrConflict
	}
	if len(raw) > 64<<10 || validation.JSONDocument(raw) != nil {
		return item, ErrInvalid
	}
	var rules []map[string]any
	if wire.Decode(raw, &rules) != nil || len(rules) > 64 {
		return item, ErrInvalid
	}
	outbounds := []map[string]any{}
	if item.OutboundsCipher != "" {
		if err = s.open("xnode.outbounds."+user+"."+node, item.OutboundsCipher, &outbounds); err != nil {
			return item, err
		}
	}
	own := map[string]bool{}
	for _, out := range outbounds {
		tag, _ := out["tag"].(string)
		own[tag] = true
	}
	for _, rule := range rules {
		if err = validateMemberRule(rule); err != nil {
			return item, err
		}
		tag, _ := rule["outboundTag"].(string)
		tag, err = namespaceTag(tag, identity.Username)
		if err != nil || !own[tag] {
			return item, ErrInvalid
		}
		rule["outboundTag"] = tag
		rule["user"] = []string{userMatcher(identity.ProxyUserID)}
	}
	return s.storeCustomization(ctx, item, identity, outbounds, rules, actor)
}
