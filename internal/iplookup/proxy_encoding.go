package iplookup

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

func endpointHost(raw string) string {
	raw = strings.Trim(strings.TrimSpace(raw), "\"'<>(),;")
	if strings.Contains(raw, "://") {
		u, e := url.Parse(raw)
		if e != nil {
			if at := strings.LastIndexByte(raw, '@'); at >= 0 {
				u, e = url.Parse("proxy://" + raw[at+1:])
			}
			if e != nil {
				return ""
			}
		}
		raw = u.Hostname()
	}
	if h, _, e := net.SplitHostPort(raw); e == nil {
		raw = h
	}
	raw = strings.Trim(raw, "[]")
	raw = strings.TrimSuffix(strings.ToLower(raw), ".")
	if ascii, e := idna.Lookup.ToASCII(raw); e == nil {
		raw = ascii
	} else {
		return ""
	}
	if ip := net.ParseIP(raw); ip != nil {
		if h, e := CanonicalIP(raw); e == nil {
			return h
		}
		return ""
	}
	if len(raw) > 253 || !strings.Contains(raw, ".") || strings.ContainsAny(raw, " /\\@?#:=\t\r\n") {
		return ""
	}
	labels := strings.Split(raw, ".")
	if strings.Trim(labels[len(labels)-1], "0123456789") == "" {
		return ""
	}
	for _, label := range strings.Split(raw, ".") {
		if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") || strings.Trim(label, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
			return ""
		}
	}
	return raw
}
func walkProxyConfig(v any, key string, add func(string)) {
	switch x := v.(type) {
	case map[string]any:
		for k, value := range x {
			if s, ok := value.(string); ok && (k == "server" || k == "address" || k == "add") {
				add(s)
			} else if _, ok := value.(string); !ok {
				walkProxyConfig(value, k, add)
			}
		}
	case []any:
		for _, value := range x {
			walkProxyConfig(value, key, add)
		}
	}
}
func decodeBase64(raw string) ([]byte, error) {
	raw = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, raw)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, e := enc.DecodeString(raw); e == nil && utf8.Valid(b) {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid encoding")
}
func decodeInventory(raw string) (string, bool) {
	fields := strings.Fields(raw)
	if len(fields) > 8 && len(fields[0]) <= 7 {
		var b strings.Builder
		valid := true
		for _, f := range fields {
			n, e := strconv.Atoi(f)
			if e != nil || n < 0 || n > utf8.MaxRune || !utf8.ValidRune(rune(n)) {
				valid = false
				break
			}
			b.WriteRune(rune(n))
		}
		if valid && strings.Contains(b.String(), "://") {
			return b.String(), true
		}
	}
	if !strings.Contains(raw, ":") && !strings.Contains(raw, "<") {
		if b, e := decodeBase64(raw); e == nil && (strings.Contains(string(b), "://") || json.Valid(b)) {
			return string(b), true
		}
	}
	return "", false
}
