package iplookup

import (
	"context"
	"math"
	"net/url"
	"strconv"
	"sync"
)

func (s *LiveService) traffic(ctx context.Context, asn uint32, key string) ASNTraffic {
	var result ASNTraffic
	var work sync.WaitGroup
	work.Add(3)
	go func() { defer work.Done(); result.BotHuman = s.trafficSummary(ctx, asn, "BOT_CLASS", key) }()
	go func() { defer work.Done(); result.Devices = s.trafficSummary(ctx, asn, "DEVICE_TYPE", key) }()
	go func() { defer work.Done(); result.IPVersion = s.trafficSummary(ctx, asn, "IP_VERSION", key) }()
	work.Wait()
	return result
}

func (s *LiveService) trafficSummary(ctx context.Context, asn uint32, dimension, key string) TrafficSummary {
	t := TrafficSummary{LiveMeta: LiveMeta{Status: "unconfigured", Source: "cloudflare"}, Values: map[string]float64{}}
	if key == "" {
		return t
	}
	data, err := s.liveHTTP(ctx, CloudflareID, "/radar/http/summary/"+dimension, url.Values{"asn": {asnText(asn)}, "dateRange": {"7d"}, "format": {"JSON"}}, nil, key)
	t.LiveMeta = liveMeta("cloudflare", err)
	if err != nil {
		return t
	}
	o := liveObject(data)
	if success := flag(o, "success"); success == nil || !*success {
		t.LiveMeta = liveMeta("cloudflare", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return t
	}
	r := nested(o, "result")
	meta := nested(r, "meta")
	t.ObservedAt = textField(meta, "lastUpdated")
	for _, window := range liveList(meta, "dateRange") {
		t.StartTime, t.EndTime = textField(window, "startTime"), textField(window, "endTime")
		break
	}
	allowed := map[string]bool{}
	switch dimension {
	case "BOT_CLASS":
		allowed = map[string]bool{"bot": true, "human": true, "LIKELY_AUTOMATED": true, "LIKELY_HUMAN": true}
	case "DEVICE_TYPE":
		allowed = map[string]bool{"desktop": true, "mobile": true, "other": true, "DESKTOP": true, "MOBILE": true, "OTHER": true}
	case "IP_VERSION":
		allowed = map[string]bool{"IPv4": true, "IPv6": true}
	}
	for name := range nested(r, "summary_0") {
		if !allowed[name] {
			continue
		}
		value, err := strconv.ParseFloat(textField(nested(r, "summary_0"), name), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
			t.Status = "partial"
			continue
		}
		t.Values[name] = value
	}
	if len(t.Values) == 0 {
		t.Status = "empty"
	}
	return t
}
