package iplookup

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

// liveHTTP accepts fixed source identifiers and relative endpoint paths only.
func (s *LiveService) liveHTTPRequest(ctx context.Context, source, path string, query url.Values, body any, credential string) ([]byte, error) {
	bases := map[string]string{"peeringdb": "https://www.peeringdb.com", "shodan": "https://internetdb.shodan.io", "ripe": "https://stat.ripe.net", "routeviews": "https://api.routeviews.org", "caida": "https://api.asrank.caida.org", "bgpkit": "https://api.bgpkit.com", "ipapi": "https://api.ipapi.is", "abuseipdb": "https://api.abuseipdb.com", CloudflareID: "https://api.cloudflare.com/client/v4"}
	base, ok := bases[source]
	if !ok || s.queues[source] == nil {
		return nil, &CodeError{"IP_DETAILS_UNAVAILABLE"}
	}
	// Queue wait is bounded by the caller's flight deadline; the per-call
	// budget below applies only to HTTP execution once the job is dispatched,
	// so slower shared pacing cannot starve tail calls of their budget.
	return upstreamqueue.Do(ctx, s.queues[source], func(ctx context.Context) ([]byte, error) {
		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		method := http.MethodGet
		var payload []byte
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			if err != nil {
				return nil, &CodeError{"IP_DETAILS_INVALID_RESPONSE"}
			}
			method = http.MethodPost
		}
		endpoint := base + path
		if len(query) > 0 {
			endpoint += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(callCtx, method, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, &CodeError{"IP_DETAILS_UNAVAILABLE"}
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "TXCarpool-IPLookup/1.0")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if source == "abuseipdb" {
			req.Header.Set("Key", credential)
		}
		if source == CloudflareID {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		if err := markLiveAttempt(callCtx); err != nil {
			return nil, err
		}
		res, err := s.client.Do(req)
		if err != nil {
			return nil, &CodeError{"IP_DETAILS_UNAVAILABLE"}
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode != http.StatusOK {
			code := "IP_DETAILS_UNAVAILABLE"
			if res.StatusCode == 401 || res.StatusCode == 403 {
				code = "IP_DETAILS_AUTH"
			}
			if res.StatusCode == 402 {
				code = "IP_DETAILS_PLAN_RESTRICTED"
			}
			if source == "shodan" && res.StatusCode == 404 {
				code = "IP_DETAILS_NOT_FOUND"
			}
			if res.StatusCode == 429 {
				code = "IP_DETAILS_RATE_LIMITED"
			}
			return nil, &CodeError{code}
		}
		limit := int64(2 * 1024 * 1024)
		if source == "abuseipdb" {
			limit = 16 * 1024 * 1024
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
		if err != nil || int64(len(data)) > limit || !json.Valid(data) {
			return nil, &CodeError{"IP_DETAILS_INVALID_RESPONSE"}
		}
		return data, nil
	})
}

func liveObject(data []byte) object { var o object; _ = json.Unmarshal(data, &o); return o }
func liveList(o object, key string) []object {
	var v []object
	_ = json.Unmarshal(o[key], &v)
	return v
}
func liveMeta(source string, err error) LiveMeta {
	m := LiveMeta{Status: "success", Source: source}
	if err != nil {
		m.Status, m.ErrorCode = "error", ErrorCode(err)
		if m.ErrorCode == "IP_DETAILS_PLAN_RESTRICTED" {
			m.Status = "restricted"
		}
	}
	return m
}

func integer(o object, key string) *int {
	n := numeric(o, key)
	if n == nil || *n < 0 || *n > 1e9 || *n != float64(int64(*n)) {
		return nil
	}
	v := int(*n)
	return &v
}

func (s *LiveService) credential(ctx context.Context, id string, c Config) (string, error) {
	if id != CloudflareID {
		enabled := false
		for _, p := range c.Providers {
			if p.ID == id {
				enabled = p.Enabled
			}
		}
		if !enabled {
			return "", nil
		}
	}
	return s.settings.Optional(ctx, CredentialKey(id))
}
