package xnode

import (
	"context"
	"encoding/json"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) storeCustomization(ctx context.Context, item Customization, identity Identity, outs, rules []map[string]any, actor string) (Customization, error) {
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
	shown, err := s.projectCustomization(item, identity)
	if err != nil {
		return item, err
	}
	saved, err := s.persistCustomization(ctx, actor, item)
	if err != nil {
		return item, err
	}
	saved.Outbounds, saved.Routing = shown.Outbounds, shown.Routing
	return saved, nil
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
