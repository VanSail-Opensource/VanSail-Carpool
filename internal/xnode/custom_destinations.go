package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

func (s *Service) resolvePublicProxy(ctx context.Context, address string) (string, error) {
	if ip, err := netip.ParseAddr(address); err == nil {
		if !wire.PublicProxyAddress(ip.String()) {
			return "", ErrInvalid
		}
		return ip.String(), nil
	}
	return upstreamqueue.Do(ctx, s.validationQueue, func(ctx context.Context) (string, error) {
		child, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		addresses, err := net.DefaultResolver.LookupIPAddr(child, address)
		if err != nil || len(addresses) == 0 {
			return "", ErrInvalid
		}
		for _, ip := range addresses {
			if !wire.PublicProxyAddress(ip.IP.String()) {
				return "", ErrInvalid
			}
		}
		return addresses[0].IP.String(), nil
	})
}
func preserveProxyHost(out map[string]any, host string) {
	stream, _ := out["streamSettings"].(map[string]any)
	if stream == nil {
		return
	}
	for _, field := range []string{"tlsSettings", "realitySettings"} {
		if security, _ := stream[field].(map[string]any); security != nil && security["serverName"] == nil {
			security["serverName"] = host
		}
	}
	for _, field := range []string{"wsSettings", "httpupgradeSettings", "xhttpSettings"} {
		if transport, _ := stream[field].(map[string]any); transport != nil && transport["host"] == nil {
			transport["host"] = host
		}
	}
	if transport, _ := stream["grpcSettings"].(map[string]any); transport != nil && transport["authority"] == nil {
		transport["authority"] = host
	}
}
func validateMemberRule(rule map[string]any) error {
	if !allowedFields(rule, "type", "domain", "ip", "port", "network", "protocol", "outboundTag", "user") {
		return ErrInvalid
	}
	if kind, exists := rule["type"]; exists && kind != "field" {
		return ErrInvalid
	}
	rule["type"] = "field"
	if matchers, exists := rule["user"]; exists {
		values, ok := matchers.([]any)
		if !ok || len(values) != 1 || values[0] != "" {
			return ErrInvalid
		}
	}
	for _, field := range []string{"domain", "ip", "protocol"} {
		value, exists := rule[field]
		if !exists {
			continue
		}
		items, ok := value.([]any)
		if !ok || len(items) == 0 || len(items) > 128 {
			return ErrInvalid
		}
		for _, raw := range items {
			text, ok := raw.(string)
			if !ok || text == "" || len(text) > 256 || strings.ContainsAny(text, "\x00\r\n") {
				return ErrInvalid
			}
			if field == "domain" {
				if strings.HasPrefix(text, "ext:") || strings.HasPrefix(text, "ext-domain:") {
					return ErrInvalid
				}
				if pattern, ok := strings.CutPrefix(text, "regexp:"); ok {
					if _, err := regexp.Compile(pattern); err != nil {
						return ErrInvalid
					}
				}
			}
			if field == "ip" {
				prefix, err := netip.ParsePrefix(text)
				if err == nil {
					if !wire.PublicProxyAddress(prefix.Addr().String()) {
						return ErrInvalid
					}
				} else if !wire.PublicProxyAddress(text) && !strings.HasPrefix(text, "geoip:") {
					return ErrInvalid
				}
			}
		}
	}
	if value, exists := rule["port"]; exists {
		text, ok := value.(string)
		if !ok {
			return ErrInvalid
		}
		if _, err := ParseBlockedPorts(text); err != nil {
			return err
		}
	}
	if value, exists := rule["network"]; exists {
		if value != "tcp" && value != "udp" && value != "tcp,udp" {
			return ErrInvalid
		}
	}
	return nil
}
