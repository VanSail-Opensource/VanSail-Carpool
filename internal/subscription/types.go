// Package subscription renders owner-scoped client configurations.
package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

type Proxy struct {
	ID              string          `json:"id"`
	NodeID          string          `json:"nodeId"`
	GroupIDs        []string        `json:"-"`
	Countries       []string        `json:"-"`
	Name            string          `json:"name"`
	Protocol        string          `json:"protocol"`
	Address         string          `json:"address"`
	Port            int             `json:"port"`
	Account         map[string]any  `json:"-"`
	Stream          map[string]any  `json:"-"`
	Mux             map[string]any  `json:"-"`
	MapperHost      map[string]any  `json:"-"`
	ExcludedFormats []string        `json:"-"`
	XrayTemplate    json.RawMessage `json:"-"`
}

func (p Proxy) Tag() string {
	digest := sha256.Sum256([]byte(p.ID))
	return "proxy-" + hex.EncodeToString(digest[:16])
}
func (p Proxy) Outbound() (map[string]any, error) {
	server := map[string]any{"address": p.Address, "port": p.Port}
	settings := map[string]any{}
	switch p.Protocol {
	case "vless", "vmess":
		server["users"] = []any{p.Account}
		settings["vnext"] = []any{server}
	case "trojan", "shadowsocks", "socks", "http":
		for key, value := range p.Account {
			server[key] = value
		}
		settings["servers"] = []any{server}
	case "hysteria":
		server["version"] = 2
		settings = server
	default:
		return nil, fmt.Errorf("unsupported subscription protocol")
	}
	result := map[string]any{"tag": p.Tag(), "protocol": p.Protocol, "settings": settings, "streamSettings": p.Stream}
	if len(p.Mux) > 0 {
		result["mux"] = p.Mux
	}
	result, err := connectivity.ApplyClientMapper(result, p.MapperHost, "xrayJson")
	if err != nil {
		return nil, err
	}
	result["tag"] = p.Tag()
	return result, nil
}
func RenderXray(proxies []Proxy) ([]byte, error) {
	return RenderTemplate(DefaultTemplate("xray"), proxies)
}
