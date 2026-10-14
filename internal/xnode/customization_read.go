package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"regexp"
	"strings"
)

var memberTagPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`)

func userMatcher(id int64) string {
	return fmt.Sprintf(`regexp:^user@%d:[0-9a-f]{32}@[0-9a-f]{24}$`, id)
}
func namespaceTag(tag, username string) (string, error) {
	if !memberTagPattern.MatchString(tag) || !wire.ValidName(username) {
		return "", ErrInvalid
	}
	return tag + "-" + username, nil
}
func (s *Service) customization(ctx context.Context, user, node string) (Customization, error) {
	items, err := s.repo.XNodeCustomizations(ctx, node)
	if err != nil {
		return Customization{}, err
	}
	for _, item := range items {
		if item.UserID == user {
			return item, nil
		}
	}
	return Customization{UserID: user, NodeID: node, Outbounds: json.RawMessage(`[]`), Routing: json.RawMessage(`[]`)}, nil
}
func (s *Service) OwnedCustomization(ctx context.Context, user, node string) (Customization, error) {
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return Customization{}, err
	}
	tags, _, err := s.eligibleTags(ctx, user, node)
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
	if item.OutboundsCipher == "" {
		return item, nil
	}
	var outbounds, rules []map[string]any
	if err = s.open("xnode.outbounds."+user+"."+node, item.OutboundsCipher, &outbounds); err != nil {
		return item, err
	}
	if err = s.open("xnode.routing."+user+"."+node, item.RoutingCipher, &rules); err != nil {
		return item, err
	}
	suffix := "-" + identity.Username
	for _, out := range outbounds {
		tag, _ := out["tag"].(string)
		out["tag"] = strings.TrimSuffix(tag, suffix)
		rewriteDependencies(out, func(tag string) string { return strings.TrimSuffix(tag, suffix) })
	}
	for _, rule := range rules {
		tag, _ := rule["outboundTag"].(string)
		rule["outboundTag"] = strings.TrimSuffix(tag, suffix)
		rule["user"] = []string{""}
	}
	item.Outbounds, _ = json.Marshal(outbounds)
	item.Routing, _ = json.Marshal(rules)
	return item, nil
}
