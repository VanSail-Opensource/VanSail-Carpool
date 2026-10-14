package app

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

func memberHostLink(host memberResolvedHost, keys *remnawave.ConnectionKeys) (*string, string) {
	if keys == nil {
		return nil, "CONNECTIVITY_LINK_UNAVAILABLE"
	}
	scheme := host.Protocol
	if scheme == "shadowsocks" {
		scheme = "ss"
	}
	if scheme == "hysteria" {
		scheme = "hysteria2"
	}
	if scheme != "vless" && scheme != "trojan" && scheme != "ss" && scheme != "hysteria2" {
		return nil, "CONNECTIVITY_LINK_UNAVAILABLE"
	}
	matches := map[string]bool{}
	for _, group := range [][]string{keys.EnabledKeys, keys.HiddenKeys} {
		for _, link := range group {
			parsed, err := url.Parse(link)
			if err != nil || parsed.User == nil || parsed.Scheme != scheme || parsed.Fragment != host.FinalRemark {
				continue
			}
			port, err := strconv.Atoi(parsed.Port())
			if err == nil && port == host.Port && strings.EqualFold(parsed.Hostname(), strings.Trim(host.Address, "[]")) {
				matches[link] = true
			}
		}
	}
	if len(matches) > 1 {
		return nil, "CONNECTIVITY_LINK_AMBIGUOUS"
	}
	for link := range matches {
		return &link, ""
	}
	return nil, "CONNECTIVITY_LINK_UNAVAILABLE"
}
