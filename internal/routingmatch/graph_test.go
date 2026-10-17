package routingmatch

import (
	"errors"
	"slices"
	"testing"
)

func rule(id, from, to string, domains ...string) Rule {
	return Rule{ID: id, From: from, To: to, Match: Match{Domains: domains}}
}
func TestConditionalPathsShareOrderedMatcher(t *testing.T) {
	g, err := Build([]Rule{rule("entry", "a", "b", "geosite:google", "geosite:ai"), rule("google", "b", "c", "geosite:google")}, func(s string) ([]string, error) {
		if s == "geosite:google" {
			return []string{"domain:google.com"}, nil
		}
		return []string{"domain:openai.com"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	paths, err := g.Paths()
	if err != nil {
		t.Fatal(err)
	}
	for domain, want := range map[string][]string{"www.google.com": {"a", "b", "c"}, "api.openai.com": {"a", "b"}, "example.org": {"a"}} {
		p, err := g.Trace("a", Destination{Domain: domain})
		if err != nil || !slices.Equal(p.Nodes, want) {
			t.Fatalf("%s: %+v %v", domain, p, err)
		}
		if len(want) > 1 && !slices.ContainsFunc(paths, func(p Path) bool { return slices.Equal(p.Nodes, want) }) {
			t.Fatalf("missing separated graph path %v", want)
		}
	}
}
func TestCyclesRequireOverlappingTraffic(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rules []Rule
		code  string
	}{
		{"disjoint domains", []Rule{rule("ab", "a", "b", "domain:google.com"), rule("ba", "b", "a", "domain:openai.com")}, ""},
		{"overlapping suffix", []Rule{rule("ab", "a", "b", "domain:google.com"), rule("ba", "b", "a", "full:www.google.com")}, "routing_loop"},
		{"unknown regular intersection", []Rule{rule("ab", "a", "b", "regexp:^a[0-9]+\\.test$"), rule("ba", "b", "a", "regexp:^a[2-8]+\\.test$")}, "routing_overlap_unproven"},
		{"first match shadows cycle", []Rule{rule("exit", "a", "c", "domain:google.com"), rule("ab", "a", "b", "domain:google.com"), rule("ba", "b", "a", "domain:google.com")}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := Build(tc.rules, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = g.Paths()
			var diagnostic *Diagnostic
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.As(err, &diagnostic) || diagnostic.Code != tc.code {
				t.Fatalf("got %v want %s", err, tc.code)
			}
		})
	}
}
func TestCIDRPortsAndNetworkRemainConjunctive(t *testing.T) {
	a, b := rule("ab", "a", "b"), rule("ba", "b", "a")
	a.Match.IPs = []string{"1.1.1.0/24"}
	b.Match.IPs = []string{"8.8.8.0/24"}
	g, _ := Build([]Rule{a, b}, nil)
	if _, err := g.Paths(); err != nil {
		t.Fatal(err)
	}
	b.Match.IPs = a.Match.IPs
	a.Match.Port = "80-90"
	b.Match.Port = "443"
	g, _ = Build([]Rule{a, b}, nil)
	if _, err := g.Paths(); err != nil {
		t.Fatal(err)
	}
	b.Match.Port = "80"
	a.Match.Networks = []string{"tcp"}
	b.Match.Networks = []string{"udp"}
	g, _ = Build([]Rule{a, b}, nil)
	if _, err := g.Paths(); err != nil {
		t.Fatal(err)
	}
	b.Match.Networks = a.Match.Networks
	g, _ = Build([]Rule{a, b}, nil)
	if _, err := g.Paths(); err == nil {
		t.Fatal("matching cycle accepted")
	}
}
func TestRejectUnresolvedAndUnboundedMatchers(t *testing.T) {
	for _, domain := range []string{"geoip:cn", "geosite:missing", "regexp:["} {
		if _, err := Build([]Rule{rule("ab", "a", "b", domain)}, nil); err == nil {
			t.Fatalf("accepted %s", domain)
		}
	}
	if _, err := Build([]Rule{rule("self", "a", "a")}, nil); err == nil {
		t.Fatal("self target accepted")
	}
}

func TestCountedRegexExpansionFailsBeforeProgramAllocation(t *testing.T) {
	_, err := Build([]Rule{rule("expanded", "a", "b", `regexp:(a{1000}){1000}`)}, nil)
	if err == nil {
		t.Fatal("expanding expression accepted")
	}
	g, err := Build([]Rule{rule("normal", "a", "b", `regexp:^a[0-9]{1,4}\.example$`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.Trace("a", Destination{Domain: "a123.example"})
	if err != nil || len(p.Nodes) != 2 {
		t.Fatal("bounded Go expression lost its behavior", err)
	}
}

func TestDomainCaseMatchesPinnedXraySemantics(t *testing.T) {
	g, err := Build([]Rule{rule("ab", "a", "b", "domain:GOOGLE.COM"), rule("ba", "b", "a", "full:www.google.com")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.Paths(); err == nil {
		t.Fatal("case folding hid a matching loop")
	}
	g, err = Build([]Rule{rule("ab", "a", "b", "domain:GOOGLE.COM")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	path, err := g.Trace("a", Destination{Domain: "WWW.Google.Com"})
	if err != nil || len(path.Nodes) != 2 {
		t.Fatal("native lowercase domain matching differs", err)
	}
	g, err = Build([]Rule{rule("ab", "a", "b", `regexp:^GOOGLE\.COM$`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	path, err = g.Trace("a", Destination{Domain: "GOOGLE.COM"})
	if err != nil || len(path.Nodes) != 1 {
		t.Fatal("regex pattern was incorrectly case folded", err)
	}
}
