package iplookup

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

func (s *LiveService) asn(ctx context.Context, id uint32, c Config) ASNDetails {
	s.prefetchASN(ctx, id)
	a := ASNDetails{ASN: id, Name: s.cachedName(id), Sources: map[string]LiveMeta{}}
	query := ripeQuery(id)
	query.Set("lod", "2")
	data, err := s.liveHTTP(ctx, "ripe", "/data/rir/data.json", query, nil, "")
	d := nested(liveObject(data), "data")
	meta := liveMeta("ripe:rir", err)
	meta.ObservedAt = textField(d, "latest")
	for _, r := range liveList(d, "rirs") {
		if textField(r, "resource") == asnText(id) {
			a.RIR = textField(r, "rir")
			a.RegisteredAt = textField(r, "registration")
			break
		}
	}
	if err == nil && a.RegisteredAt == "" {
		meta.Status = "partial"
	}
	a.Sources["registration"] = meta
	a.Sources["rir"] = meta
	data, err = s.liveHTTP(ctx, "ripe", "/data/routing-status/data.json", ripeQuery(id), nil, "")
	d = nested(liveObject(data), "data")
	meta = liveMeta("ripe:routing-status", err)
	meta.ObservedAt = textField(d, "query_time")
	if err == nil {
		space := nested(d, "announced_space")
		a.IPv4Prefixes = integer(nested(space, "v4"), "prefixes")
		a.IPv6Prefixes = integer(nested(space, "v6"), "prefixes")
		a.ObservedNeighbors = integer(d, "observed_neighbours")
		if a.IPv4Prefixes != nil && a.IPv6Prefixes != nil {
			v := *a.IPv4Prefixes > 0 || *a.IPv6Prefixes > 0
			a.Announced = &v
		} else {
			meta.Status = "partial"
		}
	}
	a.Sources["routing"] = meta
	for _, family := range []string{"4", "6"} {
		target := &a.IPv4Prefixes
		if family == "6" {
			target = &a.IPv6Prefixes
		}
		if *target != nil {
			continue
		}
		data, err = s.liveHTTP(ctx, "routeviews", "/asn/"+asnText(id), url.Values{"af": {family}}, nil, "")
		var prefixes []string
		if err == nil && json.Unmarshal(data, &prefixes) == nil {
			n := len(prefixes)
			*target = &n
			a.Sources["ipv"+family+"Prefixes"] = LiveMeta{Status: "success", Source: "routeviews", ObservedAt: s.now().UTC().Format(time.RFC3339)}
		}
	}
	if a.Announced == nil && a.IPv4Prefixes != nil && a.IPv6Prefixes != nil {
		v := *a.IPv4Prefixes > 0 || *a.IPv6Prefixes > 0
		a.Announced = &v
	}
	data, err = s.liveHTTP(ctx, "caida", "/v2/graphql", nil, asnMetadataQuery(id), "")
	d = nested(nested(liveObject(data), "data"), "asn")
	meta = liveMeta("caida", err)
	meta.ObservedAt = textField(nested(nested(liveObject(data), "data"), "dataset"), "date")
	if err == nil {
		degrees := nested(d, "asnDegree")
		a.Peers, a.Upstreams = integer(degrees, "peer"), integer(degrees, "provider")
		if a.RIR == "" {
			a.RIR = textField(d, "source")
			a.Sources["rir"] = meta
		}
		if a.Name == "" {
			a.Name = textField(d, "asnName")
			s.rememberName(id, a.Name)
		}
		if a.Peers == nil || a.Upstreams == nil {
			meta.Status = "partial"
		}
	}
	a.Sources["relationships"] = meta
	key, keyErr := s.credential(ctx, CloudflareID, c)
	if keyErr != nil {
		key = ""
	}
	if key != "" && (a.Name == "" || a.RIR == "") {
		data, err = s.liveHTTP(ctx, CloudflareID, "/radar/entities/asns/"+asnText(id), nil, nil, key)
		if err == nil {
			d = nested(nested(liveObject(data), "result"), "asn")
			if a.Name == "" {
				a.Name = textField(d, "name")
			}
			if a.RIR == "" {
				a.RIR = textField(d, "source")
				a.Sources["rir"] = LiveMeta{Status: "success", Source: "cloudflare", ObservedAt: s.now().UTC().Format(time.RFC3339)}
			}
			a.Sources["metadata"] = LiveMeta{Status: "success", Source: "cloudflare", ObservedAt: s.now().UTC().Format(time.RFC3339)}
		}
	}
	if key != "" && (a.Peers == nil || a.Upstreams == nil) {
		data, err = s.liveHTTP(ctx, CloudflareID, "/radar/entities/asns/"+asnText(id)+"/rel", nil, nil, key)
		if err == nil {
			d = nested(liveObject(data), "result")
			peers, upstreams := 0, 0
			for _, r := range liveList(d, "rels") {
				if textField(r, "asn1") != asnText(id) {
					continue
				}
				switch relationship(textField(r, "rel")) {
				case "peer":
					peers++
				case "upstream":
					upstreams++
				}
			}
			if _, ok := d["rels"]; ok {
				a.Peers, a.Upstreams = &peers, &upstreams
				a.Sources["relationships"] = LiveMeta{Status: "success", Source: "cloudflare", ObservedAt: textField(nested(d, "meta"), "data_time")}
			}
		}
	}
	a.Traffic = s.traffic(ctx, id, key)
	return a
}
