package xnode

import (
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"strings"
)

// Imported endpoint options retain the host's client transport and TLS choices.
func applyEndpointOptions(proxy *subscription.Proxy, overrides map[string]any) error {
	if stream, ok := overrides["streamSettings"].(map[string]any); ok {
		for key, value := range stream {
			proxy.Stream[key] = value
		}
	}
	for _, field := range []struct {
		key    string
		target string
	}{{"securityOptions", stringField(proxy.Stream, "security") + "Settings"}, {"transportOptions", stringField(proxy.Stream, "network") + "Settings"}} {
		value, exists := overrides[field.key]
		if !exists {
			continue
		}
		options, ok := value.(map[string]any)
		if !ok {
			return ErrInvalid
		}
		target, _ := proxy.Stream[field.target].(map[string]any)
		if target == nil {
			target = map[string]any{}
			proxy.Stream[field.target] = target
		}
		for key, value := range options {
			target[key] = value
		}
	}
	legacy, _ := overrides["legacyHost"].(map[string]any)
	if legacy == nil {
		return nil
	}
	layer := stringField(legacy, "securityLayer")
	if layer == "NONE" {
		proxy.Stream["security"] = "none"
	}
	if layer == "TLS" {
		proxy.Stream["security"] = "tls"
	}
	key := stringField(proxy.Stream, "security") + "Settings"
	security, _ := proxy.Stream[key].(map[string]any)
	if security == nil {
		security = map[string]any{}
		proxy.Stream[key] = security
	}
	if value, ok := legacy["overrideSniFromAddress"].(bool); ok && value {
		security["serverName"] = proxy.Address
	}
	if value, ok := legacy["keepSniBlank"].(bool); ok && value {
		security["serverName"] = ""
	}
	for _, field := range []string{"pinnedPeerCertSha256", "verifyPeerCertByName"} {
		if value := legacy[field]; value != nil {
			security[field] = value
		}
	}
	if alpn, ok := security["alpn"].(string); ok {
		security["alpn"] = strings.Split(alpn, ",")
	}
	if extra := legacy["xhttpExtraParams"]; extra != nil {
		transport, _ := proxy.Stream["xhttpSettings"].(map[string]any)
		if transport != nil {
			if encoded, ok := extra.(string); ok {
				var object map[string]any
				if json.Unmarshal([]byte(encoded), &object) != nil {
					return ErrInvalid
				}
				extra = object
			}
			transport["extra"] = extra
		}
	}
	return nil
}
func stringField(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return value
}
