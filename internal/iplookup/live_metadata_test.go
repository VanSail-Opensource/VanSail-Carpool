package iplookup

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBlockAlwaysMasksRequestedAddressIndependentlyOfRouting(t *testing.T) {
	for _, tc := range []struct {
		ip, prefix, network string
		mask                string
	}{{"172.127.58.147", "172.127.58.0/24", "172.127.58.0", "255.255.255.0"}, {"2606:4700:4700::1111", "2606:4700:4700::/64", "2606:4700:4700::", "ffff:ffff:ffff:ffff::"}} {
		t.Run(tc.ip, func(t *testing.T) {
			s := liveTestService(t, "abuseipdb", func(r *http.Request) (int, string) {
				if r.URL.Query().Get("network") != tc.prefix || r.URL.Query().Get("maxAgeInDays") != "30" {
					t.Errorf("wrong block query: %s", r.URL.RawQuery)
				}
				return 200, fmt.Sprintf(`{"data":{"networkAddress":"%s","netmask":"%s","reportedAddress":[]}}`, tc.network, tc.mask)
			})
			r := s.block(context.Background(), tc.ip, Config{Providers: []ProviderConfig{{ID: "abuseipdb", Enabled: true}}})
			if r.Status != "success" || r.Prefix != tc.prefix || r.ReportedAddresses == nil || *r.ReportedAddresses != 0 {
				t.Fatalf("bad block: %+v", r)
			}
		})
	}
	s := liveTestService(t, "abuseipdb", func(*http.Request) (int, string) {
		return 200, `{"data":{"networkAddress":"1.1.1.0","netmask":"255.255.255.128","reportedAddress":[]}}`
	})
	if s.block(context.Background(), "1.1.1.1", Config{Providers: []ProviderConfig{{ID: "abuseipdb", Enabled: true}}}).Status != "error" {
		t.Fatal("accepted wrong returned mask")
	}
}

func TestTransferHistorySortsDeduplicatesAndDistinguishesEmptyFromUnavailable(t *testing.T) {
	for _, tc := range []struct {
		rows, status string
		count        int
	}{{`[]`, "empty", 0}, {`null`, "error", 0}, {`[{"recipientResource":"1.1.1.0/24","transferTime":"2024-01-01","sourceHolderName":"Previous operator"},{"recipientResource":"1.1.1.0/24","transferTime":"2025-01-01"},{"recipientResource":"1.1.1.0/24","transferTime":"2025-01-01"}]`, "success", 2}, {`[{"recipientResource":"1.1.1.0/24","transferTime":"invalid"}]`, "partial", 0}} {
		t.Run(tc.status+tc.rows, func(t *testing.T) {
			s := liveTestService(t, "ripe", func(*http.Request) (int, string) {
				return 200, `{"data":{"parameters":{"resource":"1.1.1.1"},"transfers":` + tc.rows + `}}`
			})
			r := s.transfers(context.Background(), "1.1.1.1")
			if r.Status != tc.status || len(r.Transfers) != tc.count {
				t.Fatalf("bad transfers: %+v", r)
			}
			if tc.count > 0 && !strings.HasPrefix(r.Transfers[0].TransferTime, "2025") {
				t.Fatal("history is not newest first")
			}
		})
	}
}

func TestASNMetadataValidatesContactsAndCoalescesWebsiteQueries(t *testing.T) {
	s := liveTestService(t, "ripe", func(*http.Request) (int, string) {
		return 200, `{"data":{"parameters":{"resource":"45671"},"abuse_contacts":[" abuse@as45671.net.au ","abuse@as45671.net.au","bad"],"latest_time":"2026-10-11"}}`
	})
	a := ASNDetails{ASN: 45671, Sources: map[string]LiveMeta{}}
	s.asnContacts(context.Background(), &a)
	if len(a.AbuseContacts) != 1 || a.Sources["abuseContacts"].Status != "partial" {
		t.Fatalf("invalid contact normalization: %+v", a)
	}
	var calls atomic.Int32
	s = liveTestService(t, "peeringdb", func(r *http.Request) (int, string) {
		calls.Add(1)
		if r.URL.Query().Get("asn__in") != "45671" {
			t.Error("website query not scoped to ASN")
		}
		return 200, `{"data":[{"asn":45671,"website":"https://www.serversaustralia.com.au"},{"asn":999,"website":"https://other.example"}]}`
	})
	for i := 0; i < 2; i++ {
		sites, meta := s.websites(context.Background(), []uint32{45671})
		if sites[45671] != "https://www.serversaustralia.com.au" || meta.Status != "partial" {
			t.Fatal("mismatched ASN data accepted")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("identical website queries bypassed duplicate-query limits")
	}
	for _, raw := range []string{"javascript:alert(1)", "data:text/html,hi", "https://user:secret@example.com", "//example.com"} {
		if safeCompanyWebsite(raw) != "" {
			t.Errorf("unsafe website accepted: %s", raw)
		}
	}
}

func TestLegacyBlockProjectionDoesNotRelabelOrUpgradeEvidence(t *testing.T) {
	r := CompactLiveDetails(LiveDetails{IP: "172.127.58.147", Block: AbuseBlock{Prefix: "172.124.0.0/14", ReportedAddresses: integerValue(1)}})
	if r.Block.Prefix != "" || r.Block.ReportedAddresses != nil || r.Block.ErrorCode != "IP_DETAILS_LEGACY" {
		t.Fatal("legacy broad block displayed as /24")
	}
}

func integerValue(value int) *int { return &value }
