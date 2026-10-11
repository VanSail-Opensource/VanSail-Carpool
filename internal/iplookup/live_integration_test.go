package iplookup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type liveConfigSettings map[string]string

func (s liveConfigSettings) Optional(_ context.Context, key string) (string, error) {
	return s[key], nil
}

type liveResolver struct{ absent bool }

func (r liveResolver) LookupAddr(context.Context, string) ([]string, error) {
	if r.absent {
		return nil, &net.DNSError{IsNotFound: true}
	}
	return []string{"one.one.one.one.", "one.one.one.one."}, nil
}

func integratedLiveService(t *testing.T, fallback bool) (*LiveService, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	queues := map[string]*upstreamqueue.Queue{}
	for _, id := range QueueIDs() {
		q, err := upstreamqueue.New(upstreamqueue.Config{Name: id, Capacity: 32})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.Start(ctx); err != nil {
			t.Fatal(err)
		}
		queues[id] = q
		t.Cleanup(func() { _ = q.Shutdown(context.Background()) })
	}
	settings := liveConfigSettings{SettingKey: `{"enabled":true,"lookupFeeTxb":"0","refreshFeeTxb":"0","providers":[{"id":"ipapi","enabled":true},{"id":"abuseipdb","enabled":true}]}`, CredentialKey("ipapi"): "secret", CredentialKey("abuseipdb"): "secret", CredentialKey(CloudflareID): "secret"}
	s := NewLiveService(settings, queues)
	s.resolver = liveResolver{}
	scores, hosts := &atomic.Int32{}, &atomic.Int32{}
	s.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		status := 200
		switch {
		case strings.Contains(r.URL.Path, "network-info"):
			body = `{"data":{"prefix":"1.1.1.0/24"}}`
		case strings.Contains(r.URL.Path, "looking-glass"):
			if fallback {
				status = 500
			} else {
				body = `{"data":{"latest_time":"2026-10-10","rrcs":[{"rrc":"RRC01","scope":"LINX","peers":[{"prefix":"1.1.1.0/24","as_path":"174 13335 13335","latest_time":"2026-10-10"},{"prefix":"1.1.1.0/24","as_path":"3257 48266","latest_time":"2026-10-10"}]}]}}`
			}
		case strings.HasPrefix(r.URL.Path, "/prefix/"):
			body = `[{"prefix":"1.0.0.0/8","reporting_peers":[{"as_path":"999 888"}]},{"prefix":"1.1.1.0/24","reporting_peers":[{"as_path":"174 13335","collector":"route-views6","timestamp":"2026-10-10"}]}]`
		case strings.Contains(r.URL.Path, "/rir/"):
			id := strings.TrimPrefix(r.URL.Query().Get("resource"), "AS")
			body = fmt.Sprintf(`{"data":{"latest":"2026-10-09","rirs":[{"resource":"%s","rir":"ARIN","registration":"2010-07-14"}]}}`, id)
		case strings.Contains(r.URL.Path, "routing-status"):
			body = `{"data":{"query_time":"2026-10-10","announced_space":{"v4":{"prefixes":2},"v6":{"prefixes":0}},"observed_neighbours":3}}`
		case r.URL.Host == "api.asrank.caida.org":
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			data := map[string]any{"dataset": map[string]any{"date": "2026-10-01", "clique": []string{"174", "3257"}}, "asn": map[string]any{"asnName": "Origin", "source": "ARIN", "asnDegree": map[string]int{"peer": 0, "provider": 2}}}
			pattern := regexp.MustCompile(`(e\d+):asnLink\(asn0:"(\d+)",asn1:"(\d+)"\)`)
			for _, m := range pattern.FindAllStringSubmatch(input["query"], -1) {
				data[m[1]] = map[string]any{"relationship": "provider", "date": "2026-10-01", "asn0": map[string]string{"asn": m[2], "asnName": "Origin"}, "asn1": map[string]string{"asn": m[3], "asnName": "Transit"}}
			}
			if match := regexp.MustCompile(`asns\(asns:\[([^\]]+)\],first:`).FindStringSubmatch(input["query"]); len(match) > 0 {
				var ids []string
				_ = json.Unmarshal([]byte("["+match[1]+"]"), &ids)
				rows := []map[string]any{}
				for _, id := range ids {
					rows = append(rows, map[string]any{"node": map[string]string{"asn": id, "asnName": "Network"}})
				}
				data["asns"] = map[string]any{"edges": rows}
			}
			if match := regexp.MustCompile(`asnLinks\(asn:"(\d+)"`).FindStringSubmatch(input["query"]); len(match) > 0 {
				rows := []map[string]any{}
				neighbor := map[string]string{"13335": "174", "48266": "3257"}[match[1]]
				if neighbor != "" {
					rows = append(rows, map[string]any{"node": map[string]any{"asn0": map[string]string{"asn": match[1]}, "asn1": map[string]string{"asn": neighbor}, "relationship": "provider", "date": "2026-10-01"}})
				}
				data["asnLinks"] = map[string]any{"totalCount": len(rows), "edges": rows}
			}
			b, _ := json.Marshal(map[string]any{"data": data})
			body = string(b)
		case r.URL.Host == "api.ipapi.is":
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			if ip, ok := input["q"].(string); ok {
				scores.Add(1)
				body = fmt.Sprintf(`{"ip":%q,"company":{"abuser_score":"0 (Low)"},"asn":{"abuser_score":"0.001 (Low)"}}`, ip)
			}
		case r.URL.Host == "api.abuseipdb.com":
			body = `{"data":{"networkAddress":"1.1.1.0","reportedAddress":[]}}`
		case r.URL.Host == "internetdb.shodan.io":
			hosts.Add(1)
			body = fmt.Sprintf(`{"ip":%q,"hostnames":["one.one.one.one"],"ports":[443,53]}`, strings.TrimPrefix(r.URL.Path, "/"))
		case strings.Contains(r.URL.Path, "/radar/http/summary/"):
			values := map[string]string{"bot": "12.3", "human": "87.7"}
			if strings.HasSuffix(r.URL.Path, "DEVICE_TYPE") {
				values = map[string]string{"mobile": "52.4", "desktop": "46.7", "other": "0.9"}
			}
			if strings.HasSuffix(r.URL.Path, "IP_VERSION") {
				values = map[string]string{"IPv4": "65.1", "IPv6": "34.9"}
			}
			b, _ := json.Marshal(map[string]any{"success": true, "result": map[string]any{"summary_0": values, "meta": map[string]any{"lastUpdated": "2026-10-10", "dateRange": []map[string]string{{"startTime": "2026-10-03", "endTime": "2026-10-10"}}}}})
			body = string(b)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	return s, scores, hosts
}

func TestLiveEnrichmentKeepsRequestedIPsSeparate(t *testing.T) {
	s, scores, hosts := integratedLiveService(t, false)
	for _, ip := range []string{"1.1.1.9", "1.1.1.8"} {
		r, err := s.Details(context.Background(), "member", ip)
		if err != nil {
			t.Fatal(err)
		}
		if r.IP != ip || len(r.Topology.Origins) != 2 || len(r.ASNs) != 2 || len(r.Topology.Paths) != 0 {
			t.Fatalf("topology/origins: %+v", r.Topology)
		}
		if r.Scores.Company.Ratio == nil || *r.Scores.Company.Ratio != 0 || r.Block.ReportedAddresses == nil || *r.Block.ReportedAddresses != 0 || len(r.PTR.Domains) != 1 || len(r.Shodan.Ports) != 2 {
			t.Fatal("unknown/zero data or observation lost")
		}
		if r.ASNs[0].RegisteredAt != "2010-07-14" || *r.ASNs[0].IPv6Prefixes != 0 || *r.ASNs[0].Peers != 0 || r.ASNs[0].Traffic.Devices.Values["other"] != 0.9 {
			t.Fatalf("ASN: %+v", r.ASNs[0])
		}
		encoded, err := json.Marshal(r)
		if err != nil || strings.Contains(string(encoded), "secret") {
			t.Fatal("invalid or sensitive public projection")
		}
	}
	if scores.Load() != 2 || hosts.Load() != 2 {
		t.Fatal("live data reused between neighboring IPs")
	}
}

func TestLiveTopologyFallsBackToIndependentCollector(t *testing.T) {
	s, _, _ := integratedLiveService(t, true)
	s.resolver = liveResolver{absent: true}
	r, err := s.Details(context.Background(), "member", "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Topology.Source != "routeviews" || r.Topology.Prefix != "1.1.1.0/24" || len(r.Topology.Nodes) != 2 || r.PTR.Status != "empty" {
		t.Fatalf("fallback: %+v PTR=%+v", r.Topology, r.PTR)
	}
}
