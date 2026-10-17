package xnode

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/validation"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) SaveOwnedOutbounds(ctx context.Context, user, node string, revision int64, raw json.RawMessage) (Customization, error) {
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
	return s.storeCustomization(ctx, item, outbounds, rules)
}
func (s *Service) SaveOwnedRouting(ctx context.Context, user, node string, revision int64, raw json.RawMessage) (Customization, error) {
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
	return s.storeCustomization(ctx, item, outbounds, rules)
}
func (s *Service) storeCustomization(ctx context.Context, item Customization, outs, rules []map[string]any) (Customization, error) {
	raw, err := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": append([]map[string]any{{"tag": "direct", "protocol": "freedom"}}, outs...), "routing": map[string]any{"rules": rules}})
	if err != nil {
		return item, err
	}
	tags := []string{}
	for _, out := range outs {
		tag, _ := out["tag"].(string)
		tags = append(tags, tag)
	}
	if !ValidateCompiled(wire.Config{Xray: raw, Users: []wire.User{}, MemberOutbounds: tags}).Valid {
		return item, ErrInvalid
	}
	item.OutboundsCipher, err = s.seal("xnode.outbounds."+customizationOwner(item)+"."+item.NodeID, outs)
	if err != nil {
		return item, err
	}
	item.RoutingCipher, err = s.seal("xnode.routing."+customizationOwner(item)+"."+item.NodeID, rules)
	if err != nil {
		return item, err
	}
	if err = s.checkTagOwnership(ctx, item.NodeID, nil, nil, &item); err != nil {
		return item, err
	}
	if _, err = s.compile(ctx, item.NodeID, nil, nil, nil, &item); err != nil {
		return item, err
	}
	if err = s.repo.SaveXNodeCustomization(ctx, item); err != nil {
		return item, err
	}
	return s.OwnedCustomization(ctx, customizationOwner(item), item.NodeID)
}
func rewriteDependencies(out map[string]any, rewrite func(string) string) {
	if proxy, _ := out["proxySettings"].(map[string]any); proxy != nil {
		if tag, ok := proxy["tag"].(string); ok {
			proxy["tag"] = rewrite(tag)
		}
	}
	if stream, _ := out["streamSettings"].(map[string]any); stream != nil {
		if socket, _ := stream["sockopt"].(map[string]any); socket != nil {
			if tag, ok := socket["dialerProxy"].(string); ok {
				socket["dialerProxy"] = rewrite(tag)
			}
		}
	}
}
