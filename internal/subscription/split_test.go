package subscription

import (
	"encoding/json"
	"net/url"
	"testing"
)

func splitFixture(id string) Proxy {
	return Proxy{ID: id, NodeID: "native", InboundTag: "xhttp", Name: id, Protocol: "vless", Address: id + ".example", Port: 443, Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"}, Stream: map[string]any{"network": "xhttp", "security": "tls", "tlsSettings": map[string]any{"serverName": id + ".example", "fingerprint": "chrome"}, "xhttpSettings": map[string]any{"path": "/shared", "mode": "auto"}}}
}

func TestSplitSessionIdentityModesAndClientFormats(t *testing.T) {
	for _, mode := range []string{"auto", "packet-up", "stream-up"} {
		t.Run(mode, func(t *testing.T) {
			up, down := splitFixture("upload"), splitFixture("download")
			options := tuningFixture(t, `{"xhttpMode":"stream-one"}`)
			up.Tuning = &options
			p, err := SplitProxy("one", mode, up, down)
			if err != nil {
				t.Fatal(err)
			}
			out, err := p.Outbound()
			stream := object(out, "streamSettings")
			download := object(object(object(stream, "xhttpSettings"), "extra"), "downloadSettings")
			if err != nil || download["address"] != "download.example" || text(object(download, "tlsSettings"), "serverName") != "download.example" {
				t.Fatal("download reference/tuning lost", out, err)
			}
			if text(object(stream, "xhttpSettings"), "mode") != mode || text(object(up.Stream, "xhttpSettings"), "mode") != "auto" {
				t.Fatal("split modified ordinary mode")
			}
			for _, format := range []string{"xray", "clash", "stash"} {
				eligible, _, err := CompatibleProxies(format, []Proxy{up, down, p})
				if err != nil || len(eligible) != 3 {
					t.Fatal(format, len(eligible), err)
				}
				if _, err = RenderTemplate(DefaultTemplate(format), eligible); err != nil {
					t.Fatal(format, err)
				}
			}
			for _, format := range []string{"native", "base64"} {
				eligible, _, err := CompatibleProxies(format, []Proxy{up, down, p})
				if err != nil || len(eligible) != 3 {
					t.Fatal(format, len(eligible), err)
				}
				if _, err = RenderNative(eligible); err != nil {
					t.Fatal(format, err)
				}
			}
			if _, err = p.SingBox(); err == nil {
				t.Fatal("invented sing-box split support")
			}
			link, err := p.NativeLink()
			parsed, _ := url.Parse(link)
			var extra map[string]any
			if err != nil || json.Unmarshal([]byte(parsed.Query().Get("extra")), &extra) != nil || object(extra, "downloadSettings")["address"] != "download.example" {
				t.Fatal("native split lost", link, err)
			}
		})
	}
	for _, change := range []func(*Proxy){func(p *Proxy) { p.PrincipalID = 7 }, func(p *Proxy) { p.NodeID = "other" }, func(p *Proxy) { p.InboundTag = "other" }, func(p *Proxy) { object(p.Stream, "xhttpSettings")["path"] = "/other" }, func(p *Proxy) { object(p.Stream, "xhttpSettings")["mode"] = "stream-one" }} {
		up, down := splitFixture("up"), splitFixture("down")
		change(&down)
		if len(SplitModes(up, down)) != 0 {
			t.Fatal("different backend session allowed")
		}
	}
}

func TestInactiveTuningIsExcludedBeforeValidationAndExport(t *testing.T) {
	o := tuningFixture(t, `{"xhttpMode":"stream-one","xhttpScMaxEachPostBytes":{"min":9,"max":1}}`)
	if problems := ValidateTuning(o); len(problems) != 0 {
		t.Fatal("inactive values validated", problems)
	}
	active := ActiveTuning(o)
	if active.XhttpScMaxEachPostBytes != nil || o.XhttpScMaxEachPostBytes == nil {
		t.Fatal("inactive value escaped or draft mutated")
	}
	p := splitFixture("ordinary")
	p.Tuning = &o
	out, err := p.Outbound()
	extra := object(object(object(out, "streamSettings"), "xhttpSettings"), "extra")
	if err != nil || extra["scMaxEachPostBytes"] != nil || extra["downloadSettings"] != nil {
		t.Fatal(out, err)
	}
}

func TestInheritedModeControlsShareApplicability(t *testing.T) {
	p := splitFixture("inherited")
	object(p.Stream, "xhttpSettings")["mode"] = "stream-one"
	o := tuningFixture(t, `{"xhttpScMaxEachPostBytes":{"min":9,"max":1}}`)
	active := ActiveTuningFor(o, []Proxy{p})
	if active.XhttpScMaxEachPostBytes != nil {
		t.Fatal("inherited stream-one retained inactive options")
	}
	for _, field := range TuningFieldsFor([]Proxy{p}) {
		if field.Key == "xhttpScMaxEachPostBytes" && (len(field.Dependencies) != 1 || len(field.Dependencies[0].InheritedValues) != 1 || field.Dependencies[0].InheritedValues[0] != "stream-one") {
			t.Fatal("inherited mode not exposed", field)
		}
	}
	mode := "packet-up"
	o.XhttpMode = &mode
	if ActiveTuningFor(o, []Proxy{p}).XhttpScMaxEachPostBytes == nil {
		t.Fatal("explicit override did not restore applicable field")
	}
}
