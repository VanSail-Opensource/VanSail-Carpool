package iplookup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

// Settings reads encrypted credentials only at the trusted execution boundary.
type Settings interface {
	Optional(context.Context, string) (string, error)
}

type httpProvider struct {
	id       string
	queue    *upstreamqueue.Queue
	settings Settings
	client   *http.Client
}

// NewHTTPProvider creates a fixed-endpoint adapter whose calls always enter its provider queue.
func NewHTTPProvider(id string, queue *upstreamqueue.Queue, settings Settings) Provider {
	return &httpProvider{id: id, queue: queue, settings: settings, client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (p *httpProvider) ID() string { return p.id }

func (p *httpProvider) Lookup(ctx context.Context, ip string, config ProviderConfig) (ProviderResult, error) {
	credential, err := p.settings.Optional(ctx, CredentialKey(p.id))
	if err != nil || credential == "" {
		return ProviderResult{}, &CodeError{"IP_LOOKUP_CREDENTIAL_REQUIRED"}
	}
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return upstreamqueue.Do(callCtx, p.queue, func(ctx context.Context) (ProviderResult, error) {
		request, err := p.request(ctx, ip, credential, config)
		if err != nil {
			return ProviderResult{}, &CodeError{"IP_LOOKUP_PROVIDER_UNAVAILABLE"}
		}
		response, err := p.client.Do(request)
		if err != nil {
			return ProviderResult{}, &CodeError{"IP_LOOKUP_PROVIDER_UNAVAILABLE"}
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			code := "IP_LOOKUP_PROVIDER_UNAVAILABLE"
			if response.StatusCode == 401 || response.StatusCode == 403 {
				code = "IP_LOOKUP_PROVIDER_AUTH"
			}
			if response.StatusCode == 429 || response.StatusCode == 402 {
				code = "IP_LOOKUP_PROVIDER_QUOTA"
			}
			return ProviderResult{}, &CodeError{code}
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
		if err != nil || len(data) > 128*1024 {
			return ProviderResult{}, &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
		}
		var body map[string]json.RawMessage
		if json.Unmarshal(data, &body) != nil || body == nil {
			return ProviderResult{}, &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
		}
		return parseProvider(p.id, ip, body)
	})
}

func (p *httpProvider) request(ctx context.Context, ip, key string, config ProviderConfig) (*http.Request, error) {
	endpoint := ""
	query := url.Values{}
	switch p.id {
	case "abuseipdb":
		endpoint = "https://api.abuseipdb.com/api/v2/check"
		query.Set("ipAddress", ip)
		query.Set("maxAgeInDays", "90")
	case "ipapi":
		endpoint = "https://api.ipapi.is/"
		query.Set("q", ip)
		query.Set("key", key)
	case "scamalytics":
		endpoint = "https://api12.scamalytics.com/v3/" + url.PathEscape(config.AccountID) + "/"
		query.Set("ip", ip)
		query.Set("key", key)
	case "maxmind":
		endpoint = "https://geoip.maxmind.com/geoip/v2.1/insights/" + url.PathEscape(ip)
	case "ipqs":
		endpoint = "https://ipqualityscore.com/api/json/ip/" + url.PathEscape(key) + "/" + url.PathEscape(ip)
		query.Set("strictness", "3")
	case "ip2location":
		endpoint = "https://api.ip2location.io/"
		query.Set("ip", ip)
		query.Set("key", key)
	default:
		return nil, &CodeError{"IP_LOOKUP_PROVIDER_UNAVAILABLE"}
	}
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if p.id == "abuseipdb" {
		req.Header.Set("Key", key)
	}
	if p.id == "maxmind" {
		req.SetBasicAuth(config.AccountID, key)
	}
	return req, nil
}
