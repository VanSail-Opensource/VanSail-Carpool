package subscription

import (
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/ghodss/yaml"
)

func (p Proxy) Clash() (map[string]any, error) {
	protocol := p.Protocol
	if protocol == "shadowsocks" {
		protocol = "ss"
	}
	if protocol == "hysteria" {
		protocol = "hysteria2"
	}
	out := map[string]any{"type": protocol, "name": p.Name, "server": p.Address, "port": p.Port, "udp": true}
	switch p.Protocol {
	case "vless", "vmess":
		out["uuid"] = p.Account["id"]
		if p.Account["flow"] != nil {
			out["flow"] = p.Account["flow"]
		}
		if p.Protocol == "vmess" {
			out["cipher"] = "auto"
			out["alterId"] = 0
		}
	case "trojan", "shadowsocks":
		out["password"] = p.Account["password"]
		if p.Protocol == "shadowsocks" {
			out["cipher"] = p.Account["method"]
		}
	case "hysteria":
		out["password"] = object(p.Stream, "hysteriaSettings")["auth"]
	default:
		return nil, fmt.Errorf("unsupported Clash protocol")
	}
	security := text(p.Stream, "security")
	if security == "tls" || security == "reality" {
		source := object(p.Stream, security+"Settings")
		out["tls"] = true
		out["servername"] = source["serverName"]
		out["client-fingerprint"] = source["fingerprint"]
		if p.Protocol == "trojan" || p.Protocol == "hysteria" {
			out["sni"] = source["serverName"]
		}
		if source["alpn"] != nil {
			out["alpn"] = source["alpn"]
		}
		if security == "reality" {
			out["reality-opts"] = map[string]any{"public-key": source["publicKey"], "short-id": source["shortId"]}
		}
	}
	network := text(p.Stream, "network")
	source := object(p.Stream, network+"Settings")
	switch network {
	case "", "tcp", "raw", "hysteria":
	case "ws":
		out["network"] = "ws"
		options := map[string]any{"path": source["path"]}
		headers := source["headers"]
		if headers == nil && source["host"] != nil {
			headers = map[string]any{"Host": source["host"]}
		}
		if headers != nil {
			options["headers"] = headers
		}
		out["ws-opts"] = options
	case "grpc":
		out["network"] = "grpc"
		out["grpc-opts"] = map[string]any{"grpc-service-name": source["serviceName"]}
	default:
		return nil, fmt.Errorf("unsupported Clash transport")
	}
	return connectivity.ApplyClientMapper(out, p.MapperHost, "mihomo")
}
func RenderClash(proxies []Proxy) ([]byte, error) {
	values := []any{}
	names := []string{}
	for _, proxy := range proxies {
		out, err := proxy.Clash()
		if err != nil {
			return nil, err
		}
		values = append(values, out)
		names = append(names, proxy.Name)
	}
	raw, err := json.Marshal(map[string]any{"proxies": values, "proxy-groups": []any{map[string]any{"name": "proxy", "type": "select", "proxies": names}}, "rules": []string{"MATCH,proxy"}})
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(raw)
}
