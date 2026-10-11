package iplookup

import (
	"context"
	"encoding/json"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (s *LiveService) lookupRPKI(ctx context.Context, ip string) RPKIResult {
	result := RPKIResult{Routes: []RPKIRoute{}}
	data, err := s.liveHTTP(ctx, "ripe", "/data/network-info/data.json", url.Values{"resource": {ip}}, nil, "")
	result.LiveMeta = liveMeta("ripe:rpki-validation", err)
	if err != nil {
		return result
	}
	d := nested(liveObject(data), "data")
	address, addressErr := netip.ParseAddr(ip)
	text := textField(d, "prefix")
	var origins []json.RawMessage
	if json.Unmarshal(d["asns"], &origins) != nil || string(d["asns"]) == "null" || len(origins) > 300 {
		result.LiveMeta = liveMeta(result.Source, &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	if text == "" && len(origins) == 0 {
		result.Status = "empty"
		result.ObservedAt = s.now().UTC().Format(time.RFC3339)
		return result
	}
	prefix, prefixErr := netip.ParsePrefix(text)
	if addressErr != nil || prefixErr != nil || !prefix.Contains(address) {
		result.LiveMeta = liveMeta(result.Source, &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	result.Prefix = prefix.Masked().String()
	seen := map[uint32]bool{}
	for _, raw := range origins {
		id, err := strconv.ParseUint(strings.TrimPrefix(textField(object{"asn": raw}, "asn"), "AS"), 10, 32)
		if err != nil || id == 0 {
			result.Status = "partial"
			continue
		}
		if !seen[uint32(id)] {
			result.Routes = append(result.Routes, RPKIRoute{ASN: uint32(id)})
			seen[uint32(id)] = true
		}
	}
	if len(result.Routes) == 0 {
		result.LiveMeta = liveMeta(result.Source, &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	sort.Slice(result.Routes, func(i, j int) bool { return result.Routes[i].ASN < result.Routes[j].ASN })
	jobs := make(chan int, len(result.Routes))
	for i := range result.Routes {
		jobs <- i
	}
	close(jobs)
	var work sync.WaitGroup
	for range min(2, len(result.Routes)) {
		work.Add(1)
		go func() {
			defer work.Done()
			for i := range jobs {
				result.Routes[i] = s.validateRPKI(ctx, result.Prefix, result.Routes[i].ASN)
			}
		}()
	}
	work.Wait()
	failed := 0
	for _, route := range result.Routes {
		if liveMetaFailed(route.LiveMeta) {
			failed++
			result.Status = "partial"
		}
	}
	if failed == len(result.Routes) {
		result.Status = "error"
		result.ErrorCode = "IP_DETAILS_UNAVAILABLE"
	}
	result.ObservedAt = s.now().UTC().Format(time.RFC3339)
	return result
}

func (s *LiveService) validateRPKI(ctx context.Context, prefix string, asn uint32) RPKIRoute {
	result := RPKIRoute{ASN: asn, Validity: "unavailable", ROAs: []RPKIROA{}}
	data, err := s.liveHTTP(ctx, "ripe", "/data/rpki-validation/data.json", url.Values{"resource": {asnText(asn)}, "prefix": {prefix}}, nil, "")
	result.LiveMeta = liveMeta("ripe:rpki-validation", err)
	if err != nil {
		return result
	}
	d := nested(liveObject(data), "data")
	if textField(d, "prefix") != prefix || strings.TrimPrefix(textField(d, "resource"), "AS") != asnText(asn) {
		result.LiveMeta = liveMeta(result.Source, &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	switch validity := textField(d, "status"); validity {
	case "valid", "invalid_asn", "invalid_length":
		result.Validity = validity
	case "unknown":
		result.Validity = "not_found"
	default:
		result.LiveMeta = liveMeta(result.Source, &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	for _, row := range liveList(d, "validating_roas") {
		id, err := strconv.ParseUint(textField(row, "origin"), 10, 32)
		p, parseErr := netip.ParsePrefix(textField(row, "prefix"))
		length := integer(row, "max_length")
		if err != nil || id == 0 || parseErr != nil || length == nil || *length < p.Bits() || *length > p.Addr().BitLen() || len(result.ROAs) >= 128 {
			continue
		}
		result.ROAs = append(result.ROAs, RPKIROA{ASN: uint32(id), Prefix: p.Masked().String(), MaxLength: *length})
	}
	result.ObservedAt = s.now().UTC().Format(time.RFC3339)
	return result
}
