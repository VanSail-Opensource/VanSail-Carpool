package subscription

import (
	"encoding/json"
	"testing"
)

func TestXHTTPPolicyFollowsMappingAndRetainsBuiltInXMUX(t *testing.T) {
	p := splitFixture("policy")
	p.Stream["sockopt"] = map[string]any{"tcpMptcp": true}
	p.Mux = map[string]any{"enabled": true}
	p.Tuning = &ConnectionOverrides{}
	p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{"xrayJson": []any{map[string]any{"op": "set", "to": "mux.enabled", "value": true}}, "mihomo": []any{map[string]any{"op": "set", "to": "smux.enabled", "value": true}, map[string]any{"op": "set", "to": "mptcp", "value": true}}}}}
	options := tuningFixture(t, `{"xmuxMaxConcurrency":{"min":16,"max":32},"muxEnabled":true}`)
	p.Tuning = &options
	out, err := p.Outbound()
	if err != nil || out["mux"] != nil || object(object(out, "streamSettings"), "sockopt")["tcpMptcp"] != false {
		t.Fatalf("Xray policy: %+v %v", out, err)
	}
	extra := object(object(object(out, "streamSettings"), "xhttpSettings"), "extra")
	if object(extra, "xmux")["maxConcurrency"] != "16-32" {
		t.Fatal("built-in XMUX lost", extra)
	}
	clash, err := p.Clash()
	if err != nil || clash["smux"] != nil || clash["mptcp"] != false {
		t.Fatalf("Mihomo policy: %+v %v", clash, err)
	}
	if object(p.Stream, "sockopt")["tcpMptcp"] != true || p.Mux["enabled"] != true {
		t.Fatal("source/server policy mutated")
	}
}

func TestMihomoFingerprintCatalogMatchesExport(t *testing.T) {
	for _, name := range mihomoFingerprints {
		p := splitFixture("fingerprint")
		o := tuningFixture(t, `{"fingerprint":"`+name+`"}`)
		p.Tuning = &o
		if problems := ValidateTuning(o); len(problems) != 0 {
			t.Fatal(name, problems)
		}
		for _, item := range TuningCompatibility(o, []Proxy{p}) {
			if item.Key == "fingerprint" && item.Format == "clash" && item.State != "applied" {
				t.Fatal(name, item)
			}
		}
		out, err := p.Clash()
		if err != nil || out["client-fingerprint"] != name {
			t.Fatal(name, out, err)
		}
	}
}

func TestRetiredTuningIsCleanedOnlyForSavedDocuments(t *testing.T) {
	raw := []byte(`{"fingerprint":"firefox","tcpFastOpen":false,"minVersion":"1.2","tcpMptcp":true,"downloadTcpFastOpen":true,"customSockopt":[]}`)
	var o ConnectionOverrides
	if json.Unmarshal(raw, &o) == nil {
		t.Fatal("new write accepted removed options")
	}
	cleaned, err := RetireTuning(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(cleaned, &o); err != nil || o.Fingerprint == nil || *o.Fingerprint != "firefox" {
		t.Fatal(string(cleaned), err)
	}
	if o.TcpFastOpen == nil {
		t.Fatal("explicit false was discarded")
	}
}

func TestClientPolicyWithNoMemberOverridesLeavesServerProjectionUntouched(t *testing.T) {
	p := splitFixture("default-policy")
	p.Stream["sockopt"] = map[string]any{"tcpMptcp": true}
	p.Mux = map[string]any{"enabled": true}
	raw, err := RenderXray([]Proxy{p})
	if err != nil {
		t.Fatal(err)
	}
	var configs []map[string]any
	if err = json.Unmarshal(raw, &configs); err != nil || len(configs) != 1 {
		t.Fatal(string(raw), err)
	}
	out := array(configs[0], "outbounds")[0].(map[string]any)
	if out["mux"] != nil || object(object(out, "streamSettings"), "sockopt")["tcpMptcp"] != false {
		t.Fatal("untuned client escaped policy", out)
	}
	internal, err := p.Outbound()
	if err != nil || object(internal, "mux")["enabled"] != true || object(object(internal, "streamSettings"), "sockopt")["tcpMptcp"] != true {
		t.Fatal("internal projection changed", internal, err)
	}
}
