package subscription

import (
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func (p Proxy) SingBox() (map[string]any, error) {
	protocol := p.Protocol
	if protocol == "hysteria" {
		protocol = "hysteria2"
	}
	out := map[string]any{"type": protocol, "tag": p.Name, "server": p.Address, "server_port": p.Port}
	switch p.Protocol {
	case "vless", "vmess":
		out["uuid"] = p.Account["id"]
		if p.Account["flow"] != nil {
			out["flow"] = p.Account["flow"]
		}
		if p.Protocol == "vmess" {
			out["security"] = "auto"
			out["alter_id"] = 0
		}
	case "trojan", "shadowsocks":
		out["password"] = p.Account["password"]
		if p.Protocol == "shadowsocks" {
			out["method"] = p.Account["method"]
		}
	case "hysteria":
		out["password"] = object(p.Stream, "hysteriaSettings")["auth"]
	default:
		return nil, fmt.Errorf("%w: sing-box protocol", ErrUnsupported)
	}
	security := text(p.Stream, "security")
	if security == "tls" || security == "reality" {
		source := object(p.Stream, security+"Settings")
		tls := map[string]any{"enabled": true, "server_name": source["serverName"]}
		if insecure, ok := source["allowInsecure"].(bool); ok {
			tls["insecure"] = insecure
		}
		if source["alpn"] != nil {
			tls["alpn"] = source["alpn"]
		}
		if source["fingerprint"] != nil {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": source["fingerprint"]}
		}
		if security == "reality" {
			tls["reality"] = map[string]any{"enabled": true, "public_key": source["publicKey"], "short_id": source["shortId"]}
		}
		out["tls"] = tls
	}
	network := text(p.Stream, "network")
	source := object(p.Stream, network+"Settings")
	switch network {
	case "", "tcp", "raw", "hysteria":
	case "ws", "httpupgrade":
		transport := map[string]any{"type": network}
		for _, key := range []string{"path", "host", "headers"} {
			if source[key] != nil {
				transport[key] = source[key]
			}
		}
		if network == "ws" {
			delete(transport, "host")
			if transport["headers"] == nil && source["host"] != nil {
				transport["headers"] = map[string]any{"Host": source["host"]}
			}
		}
		out["transport"] = transport
	case "grpc":
		out["transport"] = map[string]any{"type": "grpc", "service_name": source["serviceName"]}
	default:
		return nil, fmt.Errorf("%w: sing-box transport", ErrUnsupported)
	}
	mapped, err := connectivity.ApplyClientMapper(out, p.MapperHost, "singbox")
	if err != nil {
		return nil, err
	}
	return p.applyTuning("sing-box", mapped)
}
func RenderSingBox(proxies []Proxy) ([]byte, error) {
	return RenderTemplate(DefaultTemplate("sing-box"), proxies)
}
