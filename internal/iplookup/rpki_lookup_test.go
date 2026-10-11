package iplookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRPKIValidatesCoveringPrefixAndEveryObservedOrigin(t *testing.T) {
	for _, validity := range []string{"valid", "invalid_asn", "invalid_length", "unknown"} {
		t.Run(validity, func(t *testing.T) {
			seen := map[string]bool{}
			var mu sync.Mutex
			s := liveTestService(t, "ripe", func(r *http.Request) (int, string) {
				if strings.Contains(r.URL.Path, "network-info") {
					return 200, `{"data":{"prefix":"118.127.0.0/18","asns":[45671,"9280",45671]}}`
				}
				if r.URL.Query().Get("prefix") != "118.127.0.0/18" {
					t.Error("validated /24 rather than actual covering prefix")
				}
				id := r.URL.Query().Get("resource")
				mu.Lock()
				seen[id] = true
				mu.Unlock()
				return 200, fmt.Sprintf(`{"data":{"prefix":"118.127.0.0/18","resource":"%s","status":"%s","validating_roas":[{"origin":"45671","prefix":"118.127.0.0/18","max_length":24}]}}`, id, validity)
			})
			result := s.lookupRPKI(context.Background(), "118.127.59.0")
			if len(result.Routes) != 2 || len(seen) != 2 || result.Status != "success" {
				t.Fatalf("wrong validation: %+v", result)
			}
			if validity == "unknown" && result.Routes[0].Validity != "not_found" {
				t.Fatal("missing ROA became source failure")
			}
			r := NewReport("r", "118.127.59.0", DefaultConfig())
			RecordProvider(&r, ProviderResult{ID: "abuseipdb", Status: "success", Complete: true, Facts: Facts{NetworkType: "residential"}, Signals: Signals{Abuse: boolean(false), Datacenter: boolean(false), VPN: boolean(false), Proxy: boolean(false), Tor: boolean(false)}}, DefaultConfig())
			r.RPKI = &result
			r = Aggregate(r, time.Now())
			invalid := validity == "invalid_asn" || validity == "invalid_length"
			if (r.Verdict == "unsuitable") != invalid || !invalid && r.Verdict != "suitable" {
				t.Fatalf("wrong quality policy: %+v", r)
			}
		})
	}
}

func TestRPKIRejectsMismatchedOrUndocumentedEvidence(t *testing.T) {
	for _, body := range []string{`{"resource":"999","prefix":"1.1.1.0/24","status":"valid"}`, `{"resource":"13335","prefix":"1.0.0.0/8","status":"valid"}`, `{"resource":"13335","prefix":"1.1.1.0/24","status":"made_up"}`} {
		s := liveTestService(t, "ripe", func(*http.Request) (int, string) { return 200, `{"data":` + body + `}` })
		if result := s.validateRPKI(context.Background(), "1.1.1.0/24", 13335); result.Status != "error" || result.Validity != "unavailable" {
			t.Fatalf("accepted invalid response: %+v", result)
		}
	}
}

func TestRPKIRepairPreservesKnownInvalidOriginWhenAnotherSourceFails(t *testing.T) {
	s := liveTestService(t, "ripe", func(r *http.Request) (int, string) {
		if r.URL.Query().Get("resource") == "45671" {
			t.Error("replayed successful invalid evidence")
		}
		return 503, `{}`
	})
	previous := RPKIResult{LiveMeta: LiveMeta{Status: "partial"}, Prefix: "118.127.59.0/24", Routes: []RPKIRoute{{ASN: 45671, Validity: "invalid_asn", LiveMeta: LiveMeta{Status: "success"}}, {ASN: 9280, Validity: "unavailable", LiveMeta: LiveMeta{Status: "error"}}}}
	result := s.repairRPKI(context.Background(), "118.127.59.0", &previous)
	if result.Routes[0].Validity != "invalid_asn" || result.Status != "partial" {
		t.Fatal("outage erased confirmed invalid route")
	}
}

func TestRPKICheckpointCodecPreservesRecoveryAndLegacyOutcomes(t *testing.T) {
	r := NewReport("r", "1.1.1.1", DefaultConfig())
	r.Status = "partial"
	r.Checkpoint.RPKI.Status = "error"
	r.RPKI = &RPKIResult{LiveMeta: LiveMeta{Status: "error"}, Routes: []RPKIRoute{}}
	raw, err := EncodeReport(r)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeReport(raw)
	if err != nil || decoded.Checkpoint.RPKI.Status != "error" {
		t.Fatal("RPKI recovery lost")
	}
	PrepareRepair(&decoded)
	if decoded.Checkpoint.RPKI.Status != "queued" {
		t.Fatal("failed validation cannot retry")
	}
	public, err := json.Marshal(PublicReport(decoded))
	if err != nil || strings.Contains(string(public), "_checkpoint") {
		t.Fatal("private RPKI checkpoint leaked")
	}
	legacy, err := DecodeReport([]byte(`{"id":"old","ip":"1.1.1.1","status":"succeeded","verdict":"suitable","policyVersion":"residential-v3"}`))
	if err != nil || legacy.Verdict != "suitable" || legacy.RPKI != nil {
		t.Fatal("legacy cached policy changed")
	}
}
