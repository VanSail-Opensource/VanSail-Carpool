package subscription

import (
	"encoding/json"
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
		return nil, fmt.Errorf("unsupported sing-box protocol")
	}
	security := text(p.Stream, "security")
	if security == "tls" || security == "reality" {
		source := object(p.Stream, security+"Settings")
		tls := map[string]any{"enabled": true, "server_name": source["serverName"]}
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
		out["transport"] = transport
	case "grpc":
		out["transport"] = map[string]any{"type": "grpc", "service_name": source["serviceName"]}
	default:
		return nil, fmt.Errorf("unsupported sing-box transport")
	}
	return connectivity.ApplyClientMapper(out, p.MapperHost, "singbox")
}
func RenderSingBox(proxies []Proxy) ([]byte, error) {
	outbounds := []any{}
	names := []string{}
	for _, proxy := range proxies {
		out, err := proxy.SingBox()
		if err != nil {
			return nil, err
		}
		outbounds = append(outbounds, out)
		names = append(names, proxy.Name)
	}
	selector := map[string]any{"type": "selector", "tag": "proxy", "outbounds": names}
	outbounds = append([]any{selector}, outbounds...)
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "direct"})
	return json.Marshal(map[string]any{"outbounds": outbounds, "route": map[string]any{"final": "proxy", "auto_detect_interface": true}})
}
