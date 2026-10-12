package iplookup

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DefaultProxySources includes all requested inventories; names are editable.
func DefaultProxySources() []ProxySource {
	rows := [][3]string{
		{"proxydb", "ProxyDB", "https://www.proxydb.net/by_ip"},
		{"proxio", "Proxio", "https://raw.githubusercontent.com/proxio-io/proxy-list/refs/heads/main/all.txt"},
		{"proxifly", "Proxifly", "https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/all/data.txt"},
		{"speedx_socks5", "TheSpeedX SOCKS5", "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks5.txt"},
		{"speedx_socks4", "TheSpeedX SOCKS4", "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks4.txt"},
		{"speedx_http", "TheSpeedX HTTP", "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt"},
		{"monosans", "Monosans", "https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/all.txt"},
		{"ebrasha", "Ebrasha", "https://github.com/ebrasha/free-v2ray-public-list/blob/main/all_extracted_configs.txt"},
		{"hookzof", "Hookzof", "https://raw.githubusercontent.com/hookzof/socks5_list/refs/heads/master/proxy.txt"},
		{"hproxy", "Hproxy", "https://raw.githubusercontent.com/hproxy-com/free-proxy-list/refs/heads/main/all.txt"},
		{"proxyscrape", "ProxyScrape", "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&proxy_format=protocolipport&format=text"},
		{"au1rxx", "Au1rxx", "https://raw.githubusercontent.com/Au1rxx/free-vpn-subscriptions/refs/heads/main/output/clash.yaml"},
		{"vpngate", "VPN Gate", "https://www.vpngate.net/api/iphone/"},
	}
	out := make([]ProxySource, 0, len(rows))
	for _, r := range rows {
		category := ProxyPoolCategory
		if r[0] == "proxydb" {
			category = ProxyDatabaseCategory
		}
		out = append(out, ProxySource{ID: r[0], Name: r[1], URL: r[2], Enabled: true, Category: category})
	}
	return out
}
func defaultResolvers() []string { return []string{"1.1.1.1:53", "8.8.8.8:53", "9.9.9.9:53"} }
func normalizeSourceURL(raw string) (string, error) {
	if len(raw) > 2048 {
		return "", &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
	}
	if strings.EqualFold(u.Hostname(), "github.com") {
		pieces := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(pieces) >= 5 && pieces[2] == "blob" {
			u.Host = "raw.githubusercontent.com"
			u.Path = "/" + strings.Join(append(pieces[:2], pieces[3:]...), "/")
			u.RawQuery = ""
		}
	}
	if parsed := net.ParseIP(u.Hostname()); parsed != nil {
		if _, err := CanonicalIP(parsed.String()); err != nil {
			return "", &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
	}
	u.Fragment = ""
	return u.String(), nil
}
func normalizeEnhancedConfig(c *Config) error {
	if c.ProxySources == nil {
		c.ProxySources = DefaultProxySources()
	}
	if c.DNSResolvers == nil {
		c.DNSResolvers = defaultResolvers()
	}
	if c.ProbePorts == nil {
		c.ProbePorts = []int{443, 4430, 8443}
	}
	if c.RefreshQuotaCost < 0 || c.RefreshQuotaCost > 1000000 || len(c.ProxySources) > 64 || len(c.DNSResolvers) == 0 || len(c.DNSResolvers) > 8 || len(c.ProbePorts) > 128 {
		return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
	}
	seen := map[string]bool{}
	for i := range c.ProxySources {
		p := &c.ProxySources[i]
		p.Category = sourceCategory(p.Category)
		if p.Category != ProxyPoolCategory && p.Category != ProxyDatabaseCategory && p.Category != AbuseCategory {
			return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
		p.Name = strings.TrimSpace(p.Name)
		if p.ID == "" || len(p.ID) > 64 || strings.Trim(p.ID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" || seen[p.ID] || p.Name == "" || utf8.RuneCountInString(p.Name) > 80 {
			return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
		seen[p.ID] = true
		normalized, err := normalizeSourceURL(p.URL)
		if err != nil {
			return err
		}
		p.URL = normalized
	}
	for i, raw := range c.DNSResolvers {
		if net.ParseIP(raw) != nil {
			raw = net.JoinHostPort(raw, "53")
		}
		h, p, e := net.SplitHostPort(raw)
		n, pe := strconv.Atoi(p)
		if e != nil || pe != nil || n < 1 || n > 65535 {
			return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
		canonical, e := CanonicalIP(h)
		if e != nil {
			return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
		c.DNSResolvers[i] = net.JoinHostPort(canonical, strconv.Itoa(n))
	}
	unique := []string{}
	resolverSeen := map[string]bool{}
	for _, resolver := range c.DNSResolvers {
		if !resolverSeen[resolver] {
			unique = append(unique, resolver)
			resolverSeen[resolver] = true
		}
	}
	c.DNSResolvers = unique
	for _, p := range c.ProbePorts {
		if p < 1 || p > 65535 {
			return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
	}
	return nil
}
