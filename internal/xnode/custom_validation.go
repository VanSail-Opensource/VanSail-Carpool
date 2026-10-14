package xnode

import (
	"context"
	"math"
	"net"
	"strings"
)

func allowedFields(object map[string]any, keys ...string) bool {
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
	}
	for key := range object {
		if !allowed[key] {
			return false
		}
	}
	return true
}
func (s *Service) validateMemberOutbound(ctx context.Context, out map[string]any, names map[string]string) error {
	if !allowedFields(out, "tag", "protocol", "settings", "streamSettings", "proxySettings") {
		return ErrInvalid
	}
	protocol, _ := out["protocol"].(string)
	settings, ok := out["settings"].(map[string]any)
	if !ok {
		return ErrInvalid
	}
	var server map[string]any
	switch protocol {
	case "vless", "vmess":
		if !allowedFields(settings, "vnext") {
			return ErrInvalid
		}
		servers, _ := settings["vnext"].([]any)
		if len(servers) != 1 {
			return ErrInvalid
		}
		server, _ = servers[0].(map[string]any)
		if server == nil || !allowedFields(server, "address", "port", "users") {
			return ErrInvalid
		}
		users, _ := server["users"].([]any)
		if len(users) != 1 {
			return ErrInvalid
		}
		user, _ := users[0].(map[string]any)
		if user == nil || !allowedFields(user, "id", "encryption", "flow", "security", "alterId") {
			return ErrInvalid
		}
	case "trojan", "shadowsocks", "socks", "http":
		if !allowedFields(settings, "servers") {
			return ErrInvalid
		}
		servers, _ := settings["servers"].([]any)
		if len(servers) != 1 {
			return ErrInvalid
		}
		server, _ = servers[0].(map[string]any)
		if server == nil || !allowedFields(server, "address", "port", "password", "method", "users", "uot", "UoTVersion") {
			return ErrInvalid
		}
		if users, exists := server["users"]; exists {
			items, ok := users.([]any)
			if !ok || len(items) != 1 {
				return ErrInvalid
			}
			account, ok := items[0].(map[string]any)
			if !ok || !allowedFields(account, "user", "pass") {
				return ErrInvalid
			}
		}
	case "hysteria":
		if !allowedFields(settings, "address", "port", "version") || settings["version"] != float64(2) {
			return ErrInvalid
		}
		server = settings
	default:
		return ErrInvalid
	}
	address, _ := server["address"].(string)
	port, _ := server["port"].(float64)
	if len(address) == 0 || len(address) > 253 || strings.ContainsAny(address, "/\\\x00\r\n\t %:@") && net.ParseIP(address) == nil || port < 1 || port > 65535 || math.Trunc(port) != port {
		return ErrInvalid
	}
	if stream, exists := out["streamSettings"]; exists {
		object, ok := stream.(map[string]any)
		if !ok || validateMemberStream(object, names) != nil {
			return ErrInvalid
		}
	}
	if proxy, exists := out["proxySettings"]; exists {
		object, ok := proxy.(map[string]any)
		if !ok || !allowedFields(object, "tag", "transportLayer") {
			return ErrInvalid
		}
		tag, _ := object["tag"].(string)
		if names[tag] == "" {
			return ErrInvalid
		}
	}
	resolved, err := s.resolvePublicProxy(ctx, address)
	if err != nil {
		return err
	}
	server["address"] = resolved
	if address != resolved {
		preserveProxyHost(out, address)
	}
	return nil
}
