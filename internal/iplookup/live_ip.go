package iplookup

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"net"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
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

func blockPrefix(ip string) string {
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	bits := 64
	if address.Is4() {
		bits = 24
	}
	return netip.PrefixFrom(address, bits).Masked().String()
}

func (s *LiveService) block(ctx context.Context, ip string, c Config) AbuseBlock {
	prefix := blockPrefix(ip)
	result := AbuseBlock{LiveMeta: LiveMeta{Status: "unconfigured", Source: "abuseipdb"}, Prefix: prefix, WindowDays: 30}
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		result.Status = "empty"
		return result
	}
	result.AddressCapacity = new(big.Int).Lsh(big.NewInt(1), uint(p.Addr().BitLen()-p.Bits())).String()
	key, err := s.credential(ctx, "abuseipdb", c)
	if err != nil {
		result.LiveMeta = liveMeta("abuseipdb", &CodeError{"IP_DETAILS_UNAVAILABLE"})
		return result
	}
	if key == "" {
		return result
	}
	data, err := s.liveHTTP(ctx, "abuseipdb", "/api/v2/check-block", url.Values{"network": {prefix}, "maxAgeInDays": {"30"}}, nil, key)
	result.LiveMeta = liveMeta("abuseipdb", err)
	if err != nil {
		return result
	}
	d := nested(liveObject(data), "data")
	result.MinAddress, result.MaxAddress = textField(d, "minAddress"), textField(d, "maxAddress")
	addr, err := netip.ParseAddr(textField(d, "networkAddress"))
	if err != nil || addr != p.Masked().Addr() {
		result.LiveMeta = liveMeta("abuseipdb", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	mask, maskErr := netip.ParseAddr(textField(d, "netmask"))
	ones, bits := net.IPMask(mask.AsSlice()).Size()
	if maskErr != nil || bits != p.Addr().BitLen() || ones != p.Bits() {
		result.LiveMeta = liveMeta("abuseipdb", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	if _, ok := d["reportedAddress"]; !ok {
		result.Status = "partial"
		return result
	}
	var rows []object
	if string(d["reportedAddress"]) == "null" || json.Unmarshal(d["reportedAddress"], &rows) != nil {
		result.Status = "partial"
		return result
	}
	seen := map[string]bool{}
	for _, row := range rows {
		addr, err := netip.ParseAddr(textField(row, "ipAddress"))
		if err != nil || !p.Contains(addr) {
			result.LiveMeta = liveMeta("abuseipdb", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
			return result
		}
		seen[addr.String()] = true
		item := AbuseAddress{IP: addr.String()}
		if count := numeric(row, "numReports"); count != nil {
			item.Reports = int(*count)
		} else {
			result.Status = "partial"
		}
		if confidence := numeric(row, "abuseConfidenceScore"); confidence != nil {
			item.Score = *confidence
		} else {
			result.Status = "partial"
		}
		result.Addresses = append(result.Addresses, item)
	}
	count := len(seen)
	result.ReportedAddresses = &count
	result.ObservedAt = s.now().UTC().Format(time.RFC3339)
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
