package xnode

import (
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func optionObject(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	if text, ok := value.(string); ok {
		var decoded map[string]any
		if json.Unmarshal([]byte(text), &decoded) != nil {
			return nil, ErrInvalid
		}
		return decoded, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, ErrInvalid
	}
	return object, nil
}

func applyEndpointClient(proxy *subscription.Proxy, overrides, inbound map[string]any) error {
	host, _ := overrides["legacyHost"].(map[string]any)
	if host == nil {
		return nil
	}
	if host["xrayJsonTemplateUuid"] != nil {
		raw, err := json.Marshal(overrides["xrayTemplate"])
		if err != nil || string(raw) == "null" {
			return ErrInvalid
		}
		if _, err = subscription.PreviewTemplate(subscription.Template{Format: "xray", Document: raw}); err != nil {
			return ErrInvalid
		}
		proxy.XrayTemplate = raw
	}
	var err error
	proxy.Mux, err = optionObject(host["muxParams"])
	if err != nil {
		return err
	}
	for _, field := range []struct{ source, target string }{{"sockoptParams", "sockopt"}, {"finalMask", "finalmask"}} {
		value, err := optionObject(host[field.source])
		if err != nil {
			return err
		}
		if value != nil {
			proxy.Stream[field.target] = value
		}
	}
	if values, ok := host["excludeFromSubscriptionTypes"].([]any); ok {
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return ErrInvalid
			}
			proxy.ExcludedFormats = append(proxy.ExcludedFormats, text)
		}
	}
	// Mapper copy operations can select entire objects. Expose only this
	// client's credentials and sanitized transport, never native account lists,
	// REALITY private keys or TLS certificate/key material from the server.
	mapperInbound := map[string]any{
		"tag": inbound["tag"], "protocol": inbound["protocol"], "port": inbound["port"],
		"settings": wire.Clone(proxy.Account), "streamSettings": wire.Clone(proxy.Stream),
	}
	proxy.MapperHost = map[string]any{
		"address": proxy.Address, "port": proxy.Port, "finalRemark": proxy.Name, "protocol": proxy.Protocol,
		"transport": proxy.Stream["network"], "security": proxy.Stream["security"], "protocolOptions": proxy.Account,
		"transportOptions": proxy.Stream[stringField(proxy.Stream, "network")+"Settings"],
		"securityOptions":  proxy.Stream[stringField(proxy.Stream, "security")+"Settings"], "mux": proxy.Mux,
		"streamOverrides": map[string]any{"sockopt": proxy.Stream["sockopt"], "finalMask": proxy.Stream["finalmask"]},
		"clientOverrides": map[string]any{"mapper": host["mapper"], "serverDescription": host["serverDescription"]},
		"metadata":        map[string]any{"rawInbound": mapperInbound, "remark": host["remark"], "tags": host["tags"], "inboundTag": inbound["tag"]},
	}
	return nil
}
