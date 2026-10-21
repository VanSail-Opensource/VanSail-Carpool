package subscription

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func tuningFixture(t *testing.T, raw string) ConnectionOverrides {
	t.Helper()
	var result ConnectionOverrides
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTuningInheritanceExplicitValuesAndProtectedIdentity(t *testing.T) {
	p := Proxy{ID: "one", Name: "TLS host", Protocol: "vless", Address: "server.example", Port: 443, Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"}, Stream: map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"serverName": "admin.example", "fingerprint": "chrome"}, "sockopt": map[string]any{"tcpKeepAliveIdle": 42, "tcpFastOpen": true}}}
	before, err := p.Outbound()
	if err != nil {
		t.Fatal(err)
	}
	zero := tuningFixture(t, `{}`)
	p.Tuning = &zero
	inherit, err := p.Outbound()
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(inherit)
	if err != nil || string(a) != string(b) {
		t.Fatal("inheritance changed behavior", err)
	}
	o := tuningFixture(t, `{"fingerprint":"firefox","tcpFastOpen":false,"tcpKeepAliveIdle":0,"muxEnabled":false,"muxConcurrency":-1}`)
	if errors := ValidateTuning(o); len(errors) != 0 {
		t.Fatal(errors)
	}
	p.Tuning = &o
	p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{"xrayJson": []any{map[string]any{"op": "set", "to": "streamSettings.tlsSettings.fingerprint", "value": "edge"}, map[string]any{"op": "set", "to": "streamSettings.sockopt.tcpFastOpen", "value": true}}}}}
	out, err := p.Outbound()
	if err != nil {
		t.Fatal(err)
	}
	stream := object(out, "streamSettings")
	if text(object(stream, "tlsSettings"), "fingerprint") != "firefox" {
		t.Fatal("explicit member option did not follow administrator mapping")
	}
	socket := object(stream, "sockopt")
	if socket["tcpFastOpen"] != false || socket["tcpKeepAliveIdle"] != float64(0) || object(out, "mux")["concurrency"] != float64(-1) {
		t.Fatal("explicit values lost", out)
	}
	if text(object(stream, "tlsSettings"), "serverName") != "admin.example" || text(object(p.Stream, "tlsSettings"), "fingerprint") != "chrome" {
		t.Fatal("identity mutated", p.Stream)
	}
	link, err := p.NativeLink()
	u, _ := url.Parse(link)
	if err != nil || u.Query().Get("fp") != "firefox" || u.Query().Get("tcpFastOpen") != "" || !strings.Contains(link, p.Address) {
		t.Fatal("native allowlist", link, err)
	}
	for _, format := range []string{"xray", "sing-box", "clash", "stash", "native", "base64"} {
		if raw, err := RenderTemplate(DefaultTemplate(format), []Proxy{p}); err != nil || len(raw) == 0 {
			t.Fatal(format, err)
		}
	}
}

func TestTuningStrictTypesRangesDependenciesAndMixedHosts(t *testing.T) {
	for _, raw := range []string{`{"password":"secret"}`, `{"serverName":"changed"}`, `{"tcpFastOpen":null}`, `{"tcpFastOpen":0.5}`, `{"xhttpXPaddingBytes":{"min":null,"max":1}}`, `{"unknown":true}`} {
		var o ConnectionOverrides
		if json.Unmarshal([]byte(raw), &o) == nil {
			t.Fatal("protected/invalid accepted", raw)
		}
	}
	for _, raw := range []string{`{"muxXudpConcurrency":1025}`, `{"minVersion":"1.3","maxVersion":"1.2"}`, `{"xhttpHeaders":[{"name":"Host","value":"evil"}]}`, `{"xmuxMaxConcurrency":{"min":1,"max":2},"xmuxMaxConnections":{"min":1,"max":2}}`, `{"xhttpMode":"stream-one","downloadTcpFastOpen":false}`, `{"ssUDPOverTCPVersion":2}`, `{"singboxMuxEnabled":true}`} {
		if errors := ValidateTuning(tuningFixture(t, raw)); len(errors) == 0 {
			t.Fatal("invalid values accepted", raw)
		}
	}
	o := tuningFixture(t, `{"fingerprint":"firefox","spiderX":"/member","tcpFastOpen":0,"xmuxHKeepAlivePeriod":-1}`)
	items := TuningCompatibility(o, []Proxy{{ID: "tls", Name: "TLS", Protocol: "trojan", Address: "one.example", Port: 443, Account: map[string]any{"password": "protected"}, Stream: map[string]any{"security": "tls", "tlsSettings": map[string]any{}, "network": "tcp"}}, {ID: "ss", Name: "SS", Protocol: "shadowsocks", Address: "two.example", Port: 443, Account: map[string]any{"method": "aes-128-gcm", "password": "protected"}, Stream: map[string]any{"network": "tcp"}}})
	for _, item := range items {
		if item.Key == "fingerprint" && item.Format == "xray" && (item.AppliedHosts != 1 || item.IncompatibleHosts != 1) {
			t.Fatal("mixed endpoints", item)
		}
		if item.Key == "tcpFastOpen" && item.Format == "sing-box" && item.State != "endpoint-incompatible" {
			t.Fatal("integer TFO mapped to boolean", item)
		}
		if item.Key == "xmuxHKeepAlivePeriod" && item.Format == "stash" && item.State != "unsupported" {
			t.Fatal("invented Stash XMUX mapping", item)
		}
	}
}
