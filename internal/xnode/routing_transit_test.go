package xnode

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestRoutingCredentialsBindOwnerPeerPeriodAndGeneration(t *testing.T) {
	doc := RoutingDocument{Seed: "a private random seed never sent to clients", Dependencies: "generation-one"}
	identity := Identity{ProxyUserID: 71, CurrentPeriodID: "period-one"}
	rule := routingmatchRule{"rule-one", "source", "target"}
	endpoint := Endpoint{InboundTag: "paid"}
	base, binding := routingCredentials(doc, identity, rule, endpoint)
	for _, change := range []string{"owner", "source", "destination", "inbound", "period", "generation", "seed"} {
		t.Run(change, func(t *testing.T) {
			d, i, r, e := doc, identity, rule, endpoint
			switch change {
			case "owner":
				i.ProxyUserID++
			case "source":
				r.From = "other"
			case "destination":
				r.To = "other"
			case "inbound":
				e.InboundTag = "other"
			case "period":
				i.CurrentPeriodID = "other"
			case "generation":
				d.Dependencies = "other"
			case "seed":
				d.Seed = "other"
			}
			credentials, peer := routingCredentials(d, i, r, e)
			if credentials.UUID == base.UUID || credentials.Trojan == base.Trojan || peer.Generation == binding.Generation {
				t.Fatal("private peer credential escaped its scope")
			}
		})
	}
	rule.ID = "second-rule-same-peer"
	other, same := routingCredentials(doc, identity, rule, endpoint)
	if other.UUID != base.UUID || same.Generation != binding.Generation {
		t.Fatal("same peer duplicated authentication bindings")
	}
	view, err := json.Marshal(RoutingView{Plan: RoutingPlan{Cipher: doc.Seed, PreviousCipher: base.UUID, LegacyCipher: base.Trojan}})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{doc.Seed, base.UUID, base.Trojan} {
		if strings.Contains(string(view), secret) {
			t.Fatal("member projection exposed a private credential")
		}
	}
	matcher := regexp.MustCompile(strings.TrimPrefix(userMatcher(identity.ProxyUserID), "regexp:"))
	if !matcher.MatchString("user@71:0123456789abcdef0123456789abcdef@0123456789abcdef01234567") {
		t.Fatal("owner matching excluded authenticated transit")
	}
}
func TestRoutingTemplateValidationUsesGoRegexAndLiteralCIDRs(t *testing.T) {
	for _, raw := range []string{`{"templates":[{"id":"a","name":"A","match":{"domain":["regexp:(?=bad)"]}}]}`, `{"templates":[{"id":"a","name":"A","match":{"ip":["geoip:cn"]}}]}`, `{"templates":[{"id":"a","name":"A","match":{"port":"0-80"}}]}`} {
		if _, err := DecodeRoutingTemplates(raw); err == nil {
			t.Fatal("invalid matcher accepted")
		}
	}
	if _, err := DecodeRoutingTemplates(`{"revision":1,"templates":[{"id":"google","name":"Google","match":{"domain":["domain:google.com","regexp:^ai[0-9]+\\.example$"],"network":["tcp"]}}]}`); err != nil {
		t.Fatal(err)
	}
}
