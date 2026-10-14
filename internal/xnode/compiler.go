package xnode

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/validation"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) Compile(ctx context.Context, nodeID string) (wire.Config, error) {
	return s.compile(ctx, nodeID, nil, nil, nil, nil)
}
func (s *Service) compile(ctx context.Context, nodeID string, replacement *Configuration, shared *Snippet, fleet *GlobalConfig, member *Customization) (wire.Config, error) {
	node, settings, err := s.Node(ctx, nodeID)
	if err != nil {
		return wire.Config{}, err
	}
	owned, err := s.NativeOwner(ctx, nodeID)
	if err != nil {
		return wire.Config{}, err
	}
	if !owned {
		global, err := s.Global(ctx)
		if err != nil {
			return wire.Config{}, err
		}
		return wire.Config{Xray: json.RawMessage(`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`), Users: []wire.User{}, Telemetry: global.Telemetry}, nil
	}
	profile, err := s.repo.XNodeConfiguration(ctx, node.ConfigID)
	if err != nil {
		return wire.Config{}, err
	}
	if replacement != nil && replacement.ID == profile.ID {
		profile = *replacement
	}
	raw, err := s.vault.Decrypt("xnode.config."+profile.ID, profile.DocumentCipher)
	if err != nil {
		return wire.Config{}, err
	}
	if len(raw) > wire.MaxFrameBytes || validation.JSONDocument([]byte(raw)) != nil {
		return wire.Config{}, ErrInvalid
	}
	var native map[string]any
	if err = json.Unmarshal([]byte(raw), &native); err != nil {
		return wire.Config{}, ErrInvalid
	}
	if !validShadowsocksMethods(native) {
		return wire.Config{}, ErrInvalid
	}
	for _, id := range profile.SnippetIDs {
		snippet, err := s.repo.XNodeSnippet(ctx, id)
		if err != nil {
			return wire.Config{}, err
		}
		if shared != nil && shared.ID == id {
			snippet = *shared
		}
		text, err := s.vault.Decrypt("xnode.snippet."+id, snippet.DocumentCipher)
		if err != nil {
			return wire.Config{}, err
		}
		var objects []any
		if err = json.Unmarshal([]byte(text), &objects); err != nil {
			return wire.Config{}, ErrInvalid
		}
		if err = appendSharedSnippet(native, snippet.Kind, objects); err != nil {
			return wire.Config{}, err
		}
	}
	global, err := s.Global(ctx)
	if err != nil {
		return wire.Config{}, err
	}
	if fleet != nil {
		global = *fleet
	}
	ports, hashes, err := GlobalWire(global, native)
	if err != nil {
		return wire.Config{}, err
	}
	users, err := s.nodeUsers(ctx, node, native, global)
	if err != nil {
		return wire.Config{}, err
	}
	customizations, err := s.repo.XNodeCustomizations(ctx, nodeID)
	if err != nil {
		return wire.Config{}, err
	}
	if member != nil {
		replaced := false
		for i := range customizations {
			if customizations[i].UserID == member.UserID {
				customizations[i] = *member
				replaced = true
			}
		}
		if !replaced {
			customizations = append(customizations, *member)
		}
	}
	active := map[string]bool{}
	for _, user := range users {
		identity, err := s.repo.XNodeIdentity(ctx, int64(user.ID))
		if err != nil {
			return wire.Config{}, err
		}
		active[identity.UserID] = true
	}
	memberOutbounds, memberRules := []any{}, []any{}
	memberTags := []string{}
	for _, custom := range customizations {
		if !active[custom.UserID] {
			continue
		}
		var outs, rules []any
		if err = s.open("xnode.outbounds."+custom.UserID+"."+nodeID, custom.OutboundsCipher, &outs); err != nil {
			return wire.Config{}, err
		}
		if err = s.open("xnode.routing."+custom.UserID+"."+nodeID, custom.RoutingCipher, &rules); err != nil {
			return wire.Config{}, err
		}
		for _, raw := range outs {
			out, ok := raw.(map[string]any)
			if !ok {
				return wire.Config{}, ErrInvalid
			}
			tag, _ := out["tag"].(string)
			memberTags = append(memberTags, tag)
		}
		memberOutbounds = append(memberOutbounds, outs...)
		memberRules = append(memberRules, rules...)
	}
	outs, ok := native["outbounds"].([]any)
	if !ok {
		return wire.Config{}, ErrInvalid
	}
	outs, hasOuts, err := expandMarker(outs, "UserOutbound", memberOutbounds)
	if err != nil {
		return wire.Config{}, err
	}
	native["outbounds"] = outs
	routing, _ := native["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
	}
	rules, _ := routing["rules"].([]any)
	if rules == nil {
		rules = []any{}
	}
	rules, hasRules, err := expandMarker(rules, "UserRouting", memberRules)
	if err != nil {
		return wire.Config{}, err
	}
	if hasOuts != hasRules {
		return wire.Config{}, ErrInvalid
	}
	if len(memberOutbounds) > 0 && (!hasOuts || !hasRules) {
		return wire.Config{}, ErrInvalid
	}
	routing["rules"] = rules
	native["routing"] = routing
	nativeJSON, err := json.Marshal(native)
	if err != nil {
		return wire.Config{}, err
	}
	config := wire.Config{Xray: nativeJSON, Users: users, Providers: global.Providers, Certificates: settings.Certificates, InboundCertificates: settings.InboundCertificates, DDNS: settings.DDNS, DNS: settings.DNS, Firewall: settings.Firewall, BlockedPorts: ports, AssetHashes: hashes, Updates: global.Updates, Telemetry: global.Telemetry, Torrent: global.Torrent, MemberOutbounds: memberTags}
	if err = config.Validate(); err != nil {
		return config, ErrInvalid
	}
	if !ValidateCompiled(config).Valid {
		return config, ErrInvalid
	}
	return config, nil
}
