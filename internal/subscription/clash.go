package subscription

import (
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func yamlFormat(format string) bool { return format == "clash" || format == "stash" }

func (p Proxy) Clash() (map[string]any, error) { return p.yamlProxy("clash") }
func (p Proxy) yamlProxy(format string) (map[string]any, error) {
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
		if insecure, ok := source["allowInsecure"].(bool); ok {
			out["skip-cert-verify"] = insecure
		}
		if p.Protocol == "trojan" || p.Protocol == "hysteria" {
			out["sni"] = source["serverName"]
		}
		if source["alpn"] != nil {
			out["alpn"] = source["alpn"]
		}
		if security == "reality" {
			out["reality-opts"] = map[string]any{"public-key": source["publicKey"], "short-id": source["shortId"], "support-x25519mlkem768": true}
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
	mapper := "mihomo"
	if format == "stash" {
		mapper = "stash"
		if out["servername"] != nil {
			out["sni"] = out["servername"]
			delete(out, "servername")
		}
	}
	mapped, err := connectivity.ApplyClientMapper(out, p.MapperHost, mapper)
	if err != nil {
		return nil, err
	}
	if security == "reality" {
		opts, _ := mapped["reality-opts"].(map[string]any)
		if opts == nil {
			opts = map[string]any{}
			mapped["reality-opts"] = opts
		}
		opts["support-x25519mlkem768"] = true
	}
	return mapped, nil
}
func RenderClash(proxies []Proxy) ([]byte, error) {
	return RenderTemplate(DefaultTemplate("clash"), proxies)
}
