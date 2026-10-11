package iplookup

import (
	"encoding/csv"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"golang.org/x/net/html"
)

const maxProxyBody = 16 << 20
const maxProxyHosts = 250000

var endpointPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://[^\s<>"\x60]+|\[[0-9a-f:]+\](?::[0-9]+)?|(?:[a-z0-9_-]+\.)+[a-z0-9_-]+:[0-9]+|(?:[0-9]{1,3}\.){3}[0-9]{1,3})`)

// ExtractProxyHosts parses supported inventory formats without retaining secrets.
func ExtractProxyHosts(data []byte) ([]string, error) {
	if len(data) > maxProxyBody {
		return nil, &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
	}
	hosts := map[string]bool{}
	partial := false
	add := func(raw string) {
		h := endpointHost(raw)
		if h != "" {
			if len(hosts) >= maxProxyHosts {
				partial = true
				return
			}
			hosts[h] = true
		}
	}
	var parse func(string, int)
	parse = func(raw string, depth int) {
		raw = strings.TrimSpace(raw)
		if raw == "" || depth > 3 {
			return
		}
		if decoded, ok := decodeInventory(raw); ok {
			parse(decoded, depth+1)
			return
		}
		if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "proxies:") || strings.Contains(raw, "server:") {
			var value any
			if yaml.Unmarshal([]byte(raw), &value) == nil {
				walkProxyConfig(value, "", add)
				if len(hosts) > 0 {
					return
				}
			}
		}
		if strings.Contains(raw, "<table") || strings.HasPrefix(raw, "<!DOCTYPE") || strings.HasPrefix(raw, "<html") {
			tokenizer := html.NewTokenizer(strings.NewReader(raw))
			for {
				kind := tokenizer.Next()
				if kind == html.ErrorToken {
					break
				}
				token := tokenizer.Token()
				if kind == html.TextToken {
					add(token.Data)
				}
				for _, a := range token.Attr {
					if a.Key == "href" {
						if u, e := url.Parse(a.Val); e == nil {
							add(u.Query().Get("ip"))
						}
					}
				}
			}
			return
		}
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "*") {
				continue
			}
			if strings.Contains(line, ",") {
				r := csv.NewReader(strings.NewReader(line))
				r.FieldsPerRecord = -1
				if fields, e := r.Read(); e == nil && len(fields) >= 2 {
					add(fields[0])
					add(fields[1])
					if len(fields) > 14 {
						if b, e := decodeBase64(fields[14]); e == nil {
							parse(string(b), depth+1)
						}
					}
					continue
				}
			}
			if strings.HasPrefix(line, "remote ") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					add(fields[1])
				}
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "vmess://") {
				if b, e := decodeBase64(line[len("vmess://"):]); e == nil {
					parse(string(b), depth+1)
				}
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "ssr://") {
				if b, e := decodeBase64(line[6:]); e == nil {
					p := strings.SplitN(string(b), ":", 2)
					add(p[0])
				}
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "ss://") && !strings.Contains(line, "@") {
				payload := strings.SplitN(line[5:], "#", 2)[0]
				if b, e := decodeBase64(payload); e == nil {
					add("ss://" + string(b))
				}
				continue
			}
			matches := endpointPattern.FindAllString(line, -1)
			for _, match := range matches {
				add(match)
			}
			if len(matches) == 0 {
				add(line)
			}
		}
	}
	parse(string(data), 0)
	out := make([]string, 0, len(hosts))
	for h := range hosts {
		out = append(out, h)
	}
	sort.Strings(out)
	if partial {
		return out, &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
	}
	return out, nil
}
