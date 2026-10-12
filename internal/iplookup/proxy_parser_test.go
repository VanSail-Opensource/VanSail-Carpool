package iplookup

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"slices"
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
		{"cidr", "102.129.155.8/24\n2001:ac8:16::1234/48\n102.129.155.0/24", []string{"102.129.155.0/24", "2001:ac8:16::/48"}},
		{"mapped_cidr", "::ffff:102.129.155.8/120", []string{"102.129.155.0/24"}},
		{"invalid_cidr", "102.129.155.0/33\n2001:ac8:16::/129\n102.129.155.0/bad\n102.129.155.0/-1", []string{}},
		{"private_cidr", "127.0.0.0/8\n10.0.0.0/24\nfd00::/48\n2001:db8::/48", []string{}},
		{"cidr_csv", "102.129.155.0/24,list\n2001:ac8:16::/48,list", []string{"102.129.155.0/24", "2001:ac8:16::/48"}},
		{"cidr_config", `{"proxies":[{"server":"102.129.155.0/24"},{"address":"2001:ac8:16::/48"}]}`, []string{"102.129.155.0/24", "2001:ac8:16::/48"}},
		{"cidr_html", `<table><tr><td>102.129.155.0/24</td><td>2001:ac8:16::/48</td></tr></table>`, []string{"102.129.155.0/24", "2001:ac8:16::/48"}},
		{"cidr_base64", base64.StdEncoding.EncodeToString([]byte("102.129.155.0/24\n2001:ac8:16::/48")), []string{"102.129.155.0/24", "2001:ac8:16::/48"}},
		{"clash", "proxies:\n  - name: sample\n    server: edge.example.net\n    port: 443\n    sni: unrelated.example.org", []string{"edge.example.net"}},
		{"clash_multiple", "proxies:\n  - name: first\n    server: edge.example.net\n    port: 443\n  - name: second\n    server: second.example.net\n    port: 8443", []string{"edge.example.net", "second.example.net"}},
		{"vmess", "vmess://" + base64.StdEncoding.EncodeToString([]byte(`{"add":"edge.example.net","host":"unrelated.example.org","port":"443"}`)), []string{"edge.example.net"}},
		{"uppercase_vmess", "VMESS://" + base64.StdEncoding.EncodeToString([]byte(`{"add":"edge.example.net","port":"443"}`)), []string{"edge.example.net"}},
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
			streamed := map[string]bool{}
			if err := readInventory(context.Background(), strings.NewReader(tc.input), func(host string) { streamed[host] = true }); err != nil {
				t.Fatal(err)
			}
			streamHosts := make([]string, 0, len(streamed))
			for host := range streamed {
				streamHosts = append(streamHosts, host)
			}
			slices.Sort(streamHosts)
			if !reflect.DeepEqual(streamHosts, tc.want) {
				t.Fatalf("stream got %v want %v", streamHosts, tc.want)
			}
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
