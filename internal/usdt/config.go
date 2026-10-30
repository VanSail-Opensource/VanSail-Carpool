package usdt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"strings"
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// DecodeConfig validates encrypted settings with the same rules as the admin write.
func DecodeConfig(raw string) (Config, error) {
	if raw == "" {
		return DefaultConfig(), nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil {
		return Config{}, ErrConfiguration
	}
	// Old encrypted configurations carry a private rate. Billing now owns the
	// shared USD rate; discard this legacy field without weakening other checks.
	delete(fields, "rate")
	encoded, err := json.Marshal(fields)
	if err != nil {
		return Config{}, ErrConfiguration
	}
	var c Config
	d := json.NewDecoder(bytes.NewReader(encoded))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return Config{}, ErrConfiguration
	}
	if _, configured := fields["concurrency"]; !configured {
		for _, group := range c.LegacyGroups {
			var groupConcurrency int
			if json.Unmarshal(group["concurrency"], &groupConcurrency) == nil && groupConcurrency > 0 {
				c.Concurrency += min(groupConcurrency, 8-c.Concurrency)
			}
		}
		c.Concurrency = max(1, c.Concurrency)
	}
	if err := ValidateConfig(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// ValidateConfig normalizes addresses and rejects unsupported endpoint protocols.
func ValidateConfig(c *Config) error {
	if c.Mode == "" {
		c.Mode = "usdt"
	}
	if c.Mode != "usdt" && c.Mode != "usdc" && c.Mode != "both" {
		return ErrConfiguration
	}
	if c.Networks == nil {
		c.Networks = []NetworkConfig{}
	}
	if len(c.Name) == 0 || len(c.Name) > 64 || c.ExpiryMinutes < 5 || c.ExpiryMinutes > 120 || c.Concurrency < 1 || c.Concurrency > 8 || len(c.Networks) > 10 {
		return ErrConfiguration
	}
	c.LegacyGroups = nil
	seen := map[string]bool{}
	endpointIDs := map[string]bool{}
	for i := range c.Networks {
		n := &c.Networks[i]
		if n.Addresses == nil {
			n.Addresses = []string{}
		}
		if n.Endpoints == nil {
			n.Endpoints = []Endpoint{}
		}
		spec, ok := Network(n.Network)
		if !ok || seen[n.Network] || len(n.Addresses) > 128 || len(n.Endpoints) > 8 {
			return ErrConfiguration
		}
		seen[n.Network] = true
		if err := normalizeCurrencies(n, c.Mode); err != nil {
			return err
		}
		addresses := map[string]bool{}
		for j, raw := range n.Addresses {
			address, err := NormalizeAddress(spec, raw)
			if err != nil || addresses[address] {
				return ErrConfiguration
			}
			n.Addresses[j] = address
			addresses[address] = true
		}
		for j := range n.Endpoints {
			e := &n.Endpoints[j]
			if !identifier.MatchString(e.ID) || endpointIDs[e.ID] || len(e.APIKey) > 4096 || strings.ContainsAny(e.APIKey, "\r\n") {
				return ErrConfiguration
			}
			e.LegacyGroup = ""
			endpointIDs[e.ID] = true
			if strings.ContainsAny(e.RPCURL, "\r\n\t") || strings.ContainsAny(e.WSURL, "\r\n\t") || strings.ContainsAny(e.IndexerURL, "\r\n\t") {
				return ErrConfiguration
			}
			if e.RPCURL == "" && !e.RPCConfigured {
				return ErrConfiguration
			} else if e.RPCURL != "" && !validURL(e.RPCURL, spec.Family == "evm") {
				return ErrConfiguration
			}
			if e.WSURL != "" && !validWSURL(e.WSURL) {
				return ErrConfiguration
			}
			if e.IndexerURL == "" {
				if spec.Family == "aptos" && !e.IndexerConfigured {
					return ErrConfiguration
				}
			} else if !validURL(e.IndexerURL, false) {
				return ErrConfiguration
			}
			if e.UseWS && ((spec.Family != "evm" && spec.Family != "solana") || (e.WSURL == "" && !e.WSConfigured && !strings.HasPrefix(e.RPCURL, "ws"))) {
				return ErrConfiguration
			}
		}
		if n.Enabled && (len(n.Addresses) == 0 || len(n.Endpoints) == 0) {
			return ErrConfiguration
		}
	}
	return nil
}

func validURL(raw string, ws bool) bool {
	if strings.ContainsAny(raw, "\r\n\t") || len(raw) > 4096 {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	switch parsed.Scheme {
	case "http", "https":
		return true
	case "ws", "wss":
		return ws
	default:
		return false
	}
}
func validWSURL(raw string) bool { return strings.HasPrefix(raw, "ws") && validURL(raw, true) }

// PublicConfig deliberately exposes no URL paths, queries, credentials or API keys.
func PublicConfig(c Config) Config {
	raw, _ := json.Marshal(c)
	var result Config
	_ = json.Unmarshal(raw, &result)
	result.LegacyGroups = nil
	for i := range result.Networks {
		for j := range result.Networks[i].Endpoints {
			e := &result.Networks[i].Endpoints[j]
			e.LegacyGroup = ""
			e.RPCConfigured, e.WSConfigured, e.IndexerConfigured, e.KeyConfigured = e.RPCURL != "", e.WSURL != "" || strings.HasPrefix(e.RPCURL, "ws"), e.IndexerURL != "", e.APIKey != ""
			parsed, err := url.Parse(e.RPCURL)
			if err == nil {
				e.Host = parsed.Hostname()
			}
			e.RPCURL, e.WSURL, e.IndexerURL, e.APIKey = "", "", "", ""
		}
	}
	return result
}

func mergeSecrets(next *Config, previous Config) {
	old := map[string]Endpoint{}
	for _, n := range previous.Networks {
		for _, e := range n.Endpoints {
			old[e.ID] = e
		}
	}
	for i := range next.Networks {
		for j := range next.Networks[i].Endpoints {
			e := &next.Networks[i].Endpoints[j]
			before := old[e.ID]
			if e.RPCURL == "" {
				e.RPCURL = before.RPCURL
			}
			if e.WSURL == "" {
				e.WSURL = before.WSURL
			}
			if e.IndexerURL == "" {
				e.IndexerURL = before.IndexerURL
			}
			if e.APIKey == "" {
				e.APIKey = before.APIKey
			}
			e.RPCConfigured, e.WSConfigured, e.IndexerConfigured, e.KeyConfigured, e.Host = false, false, false, false, ""
		}
	}
}
