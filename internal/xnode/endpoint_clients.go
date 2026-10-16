package xnode

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"strings"
)

func (s *Service) endpointProxy(ctx context.Context, identity Identity, endpoint Endpoint) (subscription.Proxy, error) {
	suffix := endpoint.ID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	result := subscription.Proxy{ID: endpoint.ID, NodeID: endpoint.NodeID, Name: endpoint.Name + "-" + suffix, Address: endpoint.Address, Port: endpoint.Port}
	groups, err := s.Groups(ctx)
	if err != nil {
		return result, err
	}
	for _, group := range groups {
		for _, binding := range group.Bindings {
			if binding.NodeID == endpoint.NodeID && binding.InboundTag == endpoint.InboundTag {
				result.GroupIDs = append(result.GroupIDs, group.ID)
				break
			}
		}
	}
	node, settings, err := s.Node(ctx, endpoint.NodeID)
	if err != nil {
		return result, err
	}
	if node.CountryCode != "" {
		result.Countries = []string{node.CountryCode}
	}
	config, err := s.ResolveConfiguration(ctx, node.ConfigID)
	if err != nil {
		return result, err
	}
	var root map[string]any
	if json.Unmarshal(config.Document, &root) != nil {
		return result, ErrInvalid
	}
	inbounds, _ := root["inbounds"].([]any)
	var inbound map[string]any
	for _, raw := range inbounds {
		item, _ := raw.(map[string]any)
		if item["tag"] == endpoint.InboundTag {
			inbound = item
			break
		}
	}
	if inbound == nil {
		return result, ErrInvalid
	}
	credentials, err := loadCredentials(s.vault, identity)
	if err != nil {
		return result, err
	}
	raw, err := inboundCredential(inbound, credentials)
	if err != nil {
		return result, err
	}
	if json.Unmarshal(raw, &result.Account) != nil {
		return result, ErrInvalid
	}
	result.Protocol, _ = inbound["protocol"].(string)
	serverSettings, _ := inbound["settings"].(map[string]any)
	if result.Protocol == "shadowsocks" {
		method, _ := serverSettings["method"].(string)
		result.Account["method"] = method
		if strings.HasPrefix(method, "2022-") {
			serverKey, _ := serverSettings["password"].(string)
			key, _ := result.Account["password"].(string)
			if serverKey == "" || key == "" {
				return result, ErrInvalid
			}
			result.Account["password"] = serverKey + ":" + key
		}
	}
	if result.Protocol == "vless" {
		result.Account["encryption"] = "none"
	}
	result.Stream, err = clientStream(inbound)
	if err != nil {
		return result, err
	}
	if result.Protocol == "hysteria" {
		transport, _ := result.Stream["hysteriaSettings"].(map[string]any)
		if transport == nil {
			transport = map[string]any{"version": 2}
			result.Stream["hysteriaSettings"] = transport
		}
		transport["auth"] = result.Account["auth"]
	}
	if profile := settings.InboundCertificates[endpoint.InboundTag]; profile != "" {
		certificate := settings.Certificates[profile]
		result.Stream["security"] = "tls"
		if certificate.Config.Domain != "" {
			result.Stream["tlsSettings"] = map[string]any{"serverName": certificate.Config.Domain, "fingerprint": "chrome"}
		}
	}
	var overrides map[string]any
	if json.Unmarshal(endpoint.Overrides, &overrides) != nil {
		return result, ErrInvalid
	}
	if err = applyEndpointOptions(&result, overrides); err != nil {
		return result, err
	}
	if err = applyEndpointClient(&result, overrides, inbound); err != nil {
		return result, err
	}
	// Endpoint overrides are administrator-owned client transport/security fields.
	for key, value := range overrides {
		if key == "legacyHost" || key == "excludedGroups" || key == "allowedGroups" || key == "securityOptions" || key == "transportOptions" || key == "xrayTemplate" {
			continue
		}
		if key != "streamSettings" && key != "account" {
			return result, ErrInvalid
		}
		object, ok := value.(map[string]any)
		if !ok {
			return result, ErrInvalid
		}
		if key == "account" {
			for name, item := range object {
				if name != "flow" && name != "encryption" {
					return result, ErrInvalid
				}
				result.Account[name] = item
			}
		} else {
			for name, item := range object {
				result.Stream[name] = item
			}
		}
	}
	out, err := result.Outbound()
	if err != nil {
		return result, err
	}
	data, err := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": []any{out}})
	if err != nil {
		return result, err
	}
	if !ValidateCompiled(wire.Config{Xray: data, Users: []wire.User{}}).Valid {
		return result, ErrInvalid
	}
	return result, nil
}

// ServiceProxy is restricted to explicitly bound monitoring identities.
func (s *Service) ServiceProxy(ctx context.Context, identity Identity, endpoint Endpoint) (subscription.Proxy, error) {
	if identity.ServiceName == "" {
		return subscription.Proxy{}, ErrNotFound
	}
	access, err := s.ServiceAccess(ctx, identity)
	if err != nil {
		return subscription.Proxy{}, err
	}
	if identity.AccessDisabled {
		return subscription.Proxy{}, ErrNotFound
	}
	if access != nil {
		period, err := s.servicePeriod(ctx, identity, access)
		if err != nil {
			return subscription.Proxy{}, err
		}
		if period == nil {
			return subscription.Proxy{}, ErrNotFound
		}
	}
	bindings, err := s.serviceBindings(ctx, identity, access)
	if err != nil {
		return subscription.Proxy{}, err
	}
	for _, binding := range bindings {
		if binding.NodeID == endpoint.NodeID && binding.InboundTag == endpoint.InboundTag && binding.ValidUntil.After(s.now()) {
			return s.endpointProxy(ctx, identity, endpoint)
		}
	}
	return subscription.Proxy{}, ErrNotFound
}
