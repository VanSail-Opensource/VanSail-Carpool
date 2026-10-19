package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func text(object map[string]any, key string) string { value, _ := object[key].(string); return value }
func object(root map[string]any, key string) map[string]any {
	value, _ := root[key].(map[string]any)
	return value
}
func array(root map[string]any, key string) []any { value, _ := root[key].([]any); return value }
func (p Proxy) NativeLink() (string, error) {
	host := net.JoinHostPort(p.Address, strconv.Itoa(p.Port))
	scheme := p.Protocol
	credential := ""
	switch p.Protocol {
	case "vless":
		credential = text(p.Account, "id")
	case "trojan":
		credential = text(p.Account, "password")
	case "shadowsocks":
		scheme = "ss"
		credential = base64.RawURLEncoding.EncodeToString([]byte(text(p.Account, "method") + ":" + text(p.Account, "password")))
	case "hysteria":
		scheme = "hysteria2"
		credential = text(object(p.Stream, "hysteriaSettings"), "auth")
	case "vmess":
		value := map[string]any{"v": "2", "ps": p.Name, "add": p.Address, "port": strconv.Itoa(p.Port), "id": p.Account["id"], "aid": "0", "scy": "auto", "net": p.Stream["network"], "tls": p.Stream["security"]}
		settings := object(p.Stream, text(p.Stream, "network")+"Settings")
		value["path"] = settings["path"]
		value["host"] = settings["host"]
		tls := object(p.Stream, "tlsSettings")
		value["sni"] = tls["serverName"]
		raw, err := json.Marshal(value)
		return "vmess://" + base64.StdEncoding.EncodeToString(raw), err
	default:
		return "", fmt.Errorf("%w: native link protocol", ErrUnsupported)
	}
	if credential == "" {
		return "", fmt.Errorf("missing proxy credential")
	}
	link := url.URL{Scheme: scheme, Host: host, User: url.User(credential), Fragment: p.Name}
	query := url.Values{}
	network := text(p.Stream, "network")
	security := text(p.Stream, "security")
	if scheme != "ss" {
		query.Set("type", network)
		query.Set("security", security)
	}
	if flow := text(p.Account, "flow"); flow != "" {
		query.Set("flow", flow)
	}
	if scheme == "vless" {
		query.Set("encryption", "none")
	}
	if security == "tls" || security == "reality" {
		settings := object(p.Stream, security+"Settings")
		for _, field := range []struct{ native, link string }{{"serverName", "sni"}, {"fingerprint", "fp"}, {"publicKey", "pbk"}, {"shortId", "sid"}, {"spiderX", "spx"}, {"mldsa65Verify", "pqv"}} {
			if value := text(settings, field.native); value != "" {
				query.Set(field.link, value)
			}
		}
		alpn := []string{}
		for _, item := range array(settings, "alpn") {
			if text, ok := item.(string); ok {
				alpn = append(alpn, text)
			}
		}
		if len(alpn) > 0 {
			query.Set("alpn", strings.Join(alpn, ","))
		}
	}
	settings := object(p.Stream, network+"Settings")
	if network == "ws" && text(settings, "host") == "" {
		headers := object(settings, "headers")
		if host := text(headers, "Host"); host != "" {
			query.Set("host", host)
		} else if host = text(headers, "host"); host != "" {
			query.Set("host", host)
		}
	}
	for _, key := range []string{"path", "host", "mode", "serviceName", "authority"} {
		if value := text(settings, key); value != "" {
			query.Set(key, value)
		}
	}
	link.RawQuery = query.Encode()
	return link.String(), nil
}
func RenderNative(proxies []Proxy) ([]byte, error) {
	if len(proxies) > 4096 {
		return nil, fmt.Errorf("subscription has too many endpoints")
	}
	links := []string{}
	total := 0
	for _, proxy := range proxies {
		link, err := proxy.NativeLink()
		if err != nil {
			return nil, err
		}
		total += len(link) + 1
		if total > 8<<20 {
			return nil, fmt.Errorf("rendered subscription exceeds 8 MiB")
		}
		links = append(links, link)
	}
	return []byte(strings.Join(links, "\n")), nil
}
