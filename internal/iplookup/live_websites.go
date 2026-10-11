package iplookup

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"time"
)

type websiteRequest struct {
	done    chan struct{}
	data    []byte
	err     error
	expires time.Time
}

// websiteData coalesces requests for one minute to honor duplicate-query limits.
func (s *LiveService) websiteData(ctx context.Context, ids []uint32) ([]byte, error) {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = asnText(id)
	}
	sort.Strings(values)
	key := strings.Join(values, ",")
	s.mu.Lock()
	if s.webRequests == nil {
		s.webRequests = map[string]*websiteRequest{}
	}
	for k, v := range s.webRequests {
		select {
		case <-v.done:
			if !v.expires.After(s.now()) {
				delete(s.webRequests, k)
			}
		default:
		}
	}
	entry := s.webRequests[key]
	if entry != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-entry.done:
			return entry.data, entry.err
		}
	}
	if len(s.webRequests) >= 256 {
		s.mu.Unlock()
		return nil, &CodeError{"IP_DETAILS_BUSY"}
	}
	entry = &websiteRequest{done: make(chan struct{})}
	s.webRequests[key] = entry
	s.mu.Unlock()
	entry.data, entry.err = s.liveHTTP(ctx, "peeringdb", "/api/net", url.Values{"asn__in": {key}, "depth": {"0"}}, nil, "")
	s.mu.Lock()
	entry.expires = s.now().Add(time.Minute)
	close(entry.done)
	s.mu.Unlock()
	return entry.data, entry.err
}

func safeCompanyWebsite(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || len(raw) > 2048 {
		return ""
	}
	return parsed.String()
}

func (s *LiveService) websites(ctx context.Context, origins []uint32) (map[uint32]string, LiveMeta) {
	result := map[uint32]string{}
	if len(origins) == 0 {
		return result, LiveMeta{Status: "empty", Source: "peeringdb"}
	}
	data, err := s.websiteData(ctx, origins)
	meta := liveMeta("peeringdb", err)
	if err != nil {
		return result, meta
	}
	o := liveObject(data)
	var rows []object
	if string(o["data"]) == "null" || json.Unmarshal(o["data"], &rows) != nil {
		return result, liveMeta("peeringdb", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
	}
	allowed := map[uint32]bool{}
	for _, id := range origins {
		allowed[id] = true
	}
	for _, row := range rows {
		n := numeric(row, "asn")
		if n == nil || *n < 1 || *n > 4294967295 || *n != float64(uint32(*n)) || !allowed[uint32(*n)] {
			meta.Status = "partial"
			continue
		}
		website := textField(row, "website")
		valid := safeCompanyWebsite(website)
		if website != "" && valid == "" {
			meta.Status = "partial"
			continue
		}
		result[uint32(*n)] = valid
	}
	meta.ObservedAt = s.now().UTC().Format(time.RFC3339)
	return result, meta
}
