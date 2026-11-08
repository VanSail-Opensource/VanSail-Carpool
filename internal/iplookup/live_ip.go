package iplookup

import (
	"context"
	"errors"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func score(o object) NetworkScore {
	raw := textField(o, "abuser_score")
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return NetworkScore{}
	}
	ratio, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 1 {
		return NetworkScore{}
	}
	return NetworkScore{Ratio: &ratio, Label: strings.Trim(strings.Join(parts[1:], " "), "()")}
}

func (s *LiveService) scores(ctx context.Context, ip string, c Config) NetworkScores {
	if scores, ok := ctx.Value(qualityScoresKey{}).(*NetworkScores); ok && scores != nil {
		return *scores
	}
	result := NetworkScores{LiveMeta: LiveMeta{Status: "unconfigured", Source: "ipapi"}}
	key, err := s.credential(ctx, "ipapi", c)
	if err != nil {
		result.LiveMeta = liveMeta("ipapi", &CodeError{"IP_DETAILS_UNAVAILABLE"})
		return result
	}
	if key == "" {
		return result
	}
	data, err := s.liveHTTP(ctx, "ipapi", "/", nil, map[string]string{"q": ip, "key": key}, "")
	result.LiveMeta = liveMeta("ipapi", err)
	if err != nil {
		return result
	}
	o := liveObject(data)
	if !validReturnedIP(o, "ip", ip) || len(o["error"]) > 0 {
		result.LiveMeta = liveMeta("ipapi", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	result.ObservedAt = s.now().UTC().Format(time.RFC3339)
	result.Company, result.ASN = score(nested(o, "company")), score(nested(o, "asn"))
	if result.Company.Ratio == nil || result.ASN.Ratio == nil {
		result.Status = "partial"
	}
	return result
}

func (s *LiveService) ptr(ctx context.Context, ip string) PTRDetails {
	result := PTRDetails{Domains: []string{}}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if s.queues["dns"] == nil {
		result.LiveMeta = liveMeta("dns", &CodeError{"IP_DETAILS_UNAVAILABLE"})
		return result
	}
	domains, err := upstreamqueue.Do(ctx, s.queues["dns"], func(ctx context.Context) ([]string, error) {
		if err := markLiveAttempt(ctx); err != nil {
			return nil, err
		}
		return s.resolver.LookupAddr(ctx, ip)
	})
	result.LiveMeta = liveMeta("dns", &CodeError{"IP_DETAILS_UNAVAILABLE"})
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		result.LiveMeta = LiveMeta{Status: "empty", Source: "dns"}
		return result
	}
	if err != nil {
		return result
	}
	result.LiveMeta = LiveMeta{Status: "success", Source: "dns", ObservedAt: s.now().UTC().Format(time.RFC3339)}
	seen := map[string]bool{}
	for _, domain := range domains {
		domain = strings.TrimSuffix(strings.TrimSpace(domain), ".")
		if domain != "" && len(domain) <= 253 {
			seen[domain] = true
		}
	}
	for domain := range seen {
		result.Domains = append(result.Domains, domain)
	}
	sort.Strings(result.Domains)
	if len(result.Domains) == 0 {
		result.Status = "empty"
	}
	return result
}
