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
var positiveRate = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,12})?$`)

// DecodeConfig validates encrypted settings with the same rules as the admin write.
func DecodeConfig(raw string) (Config, error) {
	if raw == "" {
		return DefaultConfig(), nil
	}
	var c Config
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return Config{}, ErrConfiguration
	}
	if err := ValidateConfig(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// ValidateConfig normalizes addresses and rejects unsupported endpoint protocols.
func ValidateConfig(c *Config) error {
	if c.Groups == nil {
		c.Groups = []QuotaGroup{}
	}
	if c.Networks == nil {
		c.Networks = []NetworkConfig{}
	}
	if len(c.Name) == 0 || len(c.Name) > 64 || c.ExpiryMinutes < 5 || c.ExpiryMinutes > 120 || len(c.Networks) > 10 || len(c.Groups) > 32 {
		return ErrConfiguration
	}
	if c.Rate != "" && (!positiveRate.MatchString(c.Rate) || strings.Trim(c.Rate, "0.") == "") {
		return ErrConfiguration
	}
	if c.Enabled && c.Rate == "" {
		return ErrConfiguration
	}
	groups := map[string]bool{}
	for _, g := range c.Groups {
		if !identifier.MatchString(g.ID) || groups[g.ID] || g.RequestsPerSecond < 1 || g.RequestsPerSecond > 50 || g.RequestsPerMinute < 1 || g.RequestsPerMinute > 3000 || g.RequestsPerDay < 1 || g.RequestsPerDay > 1000000 || g.Concurrency < 1 || g.Concurrency > 8 {
			return ErrConfiguration
		}
		groups[g.ID] = true
	}
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
		addresses := map[string]bool{}
		for j, raw := range n.Addresses {
			address, err := NormalizeAddress(spec, raw)
			if err != nil || addresses[address] {
				return ErrConfiguration
			}
			n.Addresses[j] = address
			addresses[address] = true
		}
		for _, e := range n.Endpoints {
			if !identifier.MatchString(e.ID) || endpointIDs[e.ID] || !groups[e.Group] || len(e.APIKey) > 4096 || strings.ContainsAny(e.APIKey, "\r\n") {
				return ErrConfiguration
			}
			endpointIDs[e.ID] = true
			if !validURL(e.RPCURL, spec.Family == "evm") || (e.WSURL != "" && !validWSURL(e.WSURL)) || (e.IndexerURL != "" && !validURL(e.IndexerURL, false)) {
				return ErrConfiguration
			}
			if e.UseWS && ((spec.Family != "evm" && spec.Family != "solana") || (e.WSURL == "" && !strings.HasPrefix(e.RPCURL, "ws"))) {
				return ErrConfiguration
			}
			if spec.Family == "aptos" && e.IndexerURL == "" {
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
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Hostname() != "" && parsed.User == nil && parsed.Fragment == "" && len(raw) <= 4096 && (parsed.Scheme == "http" || parsed.Scheme == "https" || (ws && (parsed.Scheme == "ws" || parsed.Scheme == "wss")))
}
func validWSURL(raw string) bool { return strings.HasPrefix(raw, "ws") && validURL(raw, true) }

// PublicConfig deliberately exposes no URL paths, queries, credentials or API keys.
func PublicConfig(c Config) Config {
	raw, _ := json.Marshal(c)
	var result Config
	_ = json.Unmarshal(raw, &result)
	for i := range result.Networks {
		for j := range result.Networks[i].Endpoints {
			e := &result.Networks[i].Endpoints[j]
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
