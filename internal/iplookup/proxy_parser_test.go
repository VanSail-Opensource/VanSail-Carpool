package iplookup

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestExtractProxyHostsFormats(t *testing.T) {
	uri := "hysteria2://secret@edge.example.net:443?sni=unrelated.example.org"
	var decimal strings.Builder
	for _, r := range uri {
		fmt.Fprintf(&decimal, "%d\n", r)
	}
	cases := []struct {
		name, input string
		want        []string
	}{
		{"plain", "8.8.8.8:1080\n1.1.1.1:8080\n8.8.8.8:1080", []string{"1.1.1.1", "8.8.8.8"}},
		{"protocol", uri, []string{"edge.example.net"}},
		{"ipv6", "socks5://[2606:4700:4700::1111]:1080", []string{"2606:4700:4700::1111"}},
		{"clash", "proxies:\n  - name: sample\n    server: edge.example.net\n    port: 443\n    sni: unrelated.example.org", []string{"edge.example.net"}},
		{"vmess", "vmess://" + base64.StdEncoding.EncodeToString([]byte(`{"add":"edge.example.net","host":"unrelated.example.org","port":"443"}`)), []string{"edge.example.net"}},
		{"base64", base64.StdEncoding.EncodeToString([]byte(uri)), []string{"edge.example.net"}},
		{"decimal", decimal.String(), []string{"edge.example.net"}},
		{"proxydb", `<table><tbody><tr><td><a href="/?ip=8.8.8.8">8.8.8.8</a></td></tr></tbody></table>`, []string{"8.8.8.8"}},
		{"vpngate", "*vpn_servers\n#HostName,IP,Score\nvpn-server,8.8.8.8,100", []string{"8.8.8.8"}},
		{"csv_endpoint", "IP,Port\n8.8.8.8,1080", []string{"8.8.8.8"}},
		{"malformed_ipv4", "999.999.999.999:1080", []string{}},
		{"openvpn", "remote edge.example.net 1194", []string{"edge.example.net"}},
		{"private", "127.0.0.1:1080\n10.0.0.1:443\nhttps://edge.example.net:443", []string{"edge.example.net"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractProxyHosts([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
func TestProxySourceNormalizationAndValidation(t *testing.T) {
	raw := "https://github.com/owner/repo/blob/main/list.txt"
	normalized, err := normalizeSourceURL(raw)
	if err != nil || normalized != "https://raw.githubusercontent.com/owner/repo/main/list.txt" {
		t.Fatalf("normalize: %q %v", normalized, err)
	}
	for _, raw := range []string{"http://127.0.0.1/list", "ftp://example.net/list", "https://user:secret@example.net/list"} {
		if _, err := normalizeSourceURL(raw); err == nil {
			t.Fatalf("accepted unsafe URL %q", raw)
		}
	}
	c := DefaultConfig()
	c.ProxySources = append(c.ProxySources, c.ProxySources[0])
	if normalizeEnhancedConfig(&c) == nil {
		t.Fatal("accepted duplicate source")
	}
	if len(DefaultProxySources()) != 13 {
		t.Fatal("default inventories incomplete")
	}
}
