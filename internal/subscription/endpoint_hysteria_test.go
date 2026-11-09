package subscription

import (
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func hoppingProxy() Proxy {
	ports := []TuningPortRange{{Min: 443, Max: 443}, {Min: 20000, Max: 30000}, {Min: 65535, Max: 65535}}
	return Proxy{Protocol: "hysteria", Address: "2001:db8::1", Port: 443, EndpointTuning: &EndpointOptions{HysteriaPorts: &ports, HysteriaHopInterval: &TuningRange{Min: 15, Max: 45}}}
}

func TestEndpointHysteriaClientPortAndTimingFormats(t *testing.T) {
	p := hoppingProxy()
	for _, format := range []string{"xray", "sing-box", "clash", "stash"} {
		t.Run(format, func(t *testing.T) {
			out := map[string]any{"server_port": 443}
			if err := endpointHysteria(p, format, out); err != nil {
				t.Fatal(err)
			}
			switch format {
			case "xray":
				hop := object(array(object(object(out, "streamSettings"), "finalmask"), "udp")[0].(map[string]any), "settings")
				if hop["mode"] != "intervalRemote" || hop["remotePorts"] != "443,20000-30000,65535" || hop["interval"] != "15-45" {
					t.Fatal(hop)
				}
			case "sing-box":
				if !reflect.DeepEqual(out["server_ports"], []string{"443", "20000:30000", "65535"}) || out["server_port"] != nil || out["hop_interval"] != "15s" || out["hop_interval_max"] != "45s" {
					t.Fatal(out)
				}
			default:
				if out["ports"] != "443,20000-30000,65535" || format == "clash" && out["hop-interval"] != "15-45" || format == "stash" && out["hop-interval"] != nil {
					t.Fatal(out)
				}
			}
			if endpointHysteriaTimingOmitted(p, format) != (format == "stash") {
				t.Fatal("incorrect timing compatibility", format)
			}
		})
	}
	link := &url.URL{Scheme: "hysteria2", User: url.User("a/b"), Host: "[2001:db8::1]:443", RawQuery: "sni=example.com"}
	if err := endpointHysteriaNative(p, link); err != nil || !strings.Contains(link.String(), "@[2001:db8::1]:443,20000-30000,65535?") || link.RawQuery != "sni=example.com" || !endpointHysteriaTimingOmitted(p, "native") {
		t.Fatal(link, err)
	}
}

func TestEndpointHysteriaFixedTimingDoesNotRetainRandomMaximum(t *testing.T) {
	p := hoppingProxy()
	p.EndpointTuning.HysteriaHopInterval = &TuningRange{Min: 5, Max: 5}
	out := map[string]any{"hop_interval_max": "90s"}
	if err := endpointHysteria(p, "sing-box", out); err != nil || out["hop_interval"] != "5s" || out["hop_interval_max"] != nil {
		t.Fatal(out, err)
	}
	if err := endpointHysteria(p, "stash", out); err != nil || out["hop-interval"] != int64(5) || endpointHysteriaTimingOmitted(p, "stash") {
		t.Fatal(out, err)
	}
}

func TestEndpointHysteriaInheritsAndPreservesOtherMasksWithoutMutation(t *testing.T) {
	p := hoppingProxy()
	p.EndpointTuning = nil
	p.Stream = map[string]any{"finalmask": map[string]any{"tcp": []any{"untouched"}, "udp": []any{map[string]any{"type": "noise", "settings": map[string]any{"packet": "random"}}, map[string]any{"type": "udphop", "settings": map[string]any{"mode": "intervalRemote", "remotePorts": "443,65535", "interval": "15-45"}}}}}
	before, _ := json.Marshal(p.Stream)
	out := map[string]any{"streamSettings": p.Stream}
	if err := endpointHysteria(p, "xray", out); err != nil {
		t.Fatal(err)
	}
	final := object(object(out, "streamSettings"), "finalmask")
	if text(array(final, "udp")[0].(map[string]any), "type") != "noise" || len(array(final, "udp")) != 2 || len(array(final, "tcp")) != 1 || !hasEndpointHysteriaPorts(p) {
		t.Fatal(final)
	}
	after, _ := json.Marshal(p.Stream)
	if string(after) != string(before) {
		t.Fatal("rendering mutated published endpoint state")
	}
	if err := endpointHysteria(p, "clash", out); err != nil || out["ports"] != "443,65535" || out["hop-interval"] != "15-45" {
		t.Fatal(out, err)
	}
	empty := []TuningPortRange{}
	p.EndpointTuning = &EndpointOptions{HysteriaPorts: &empty}
	if err := endpointHysteria(p, "xray", out); err != nil || len(array(object(object(out, "streamSettings"), "finalmask"), "udp")) != 1 || hasEndpointHysteriaPorts(p) {
		t.Fatal(out, err)
	}
}

func TestEndpointHysteriaMalformedPortsExcludeOnlyAffectedEndpoint(t *testing.T) {
	for _, ports := range [][]TuningPortRange{{{Min: 0, Max: 443}}, {{Min: 2, Max: 1}}, {{Min: 1, Max: 65536}}} {
		p := hoppingProxy()
		p.EndpointTuning.HysteriaPorts = &ports
		if err := endpointHysteria(p, "sing-box", map[string]any{}); !errors.Is(err, ErrUnsupported) {
			t.Fatal("invalid essential ports accepted", ports, err)
		}
	}
	p := hoppingProxy()
	p.EndpointTuning.HysteriaHopInterval = &TuningRange{Min: 1, Max: 4}
	out := map[string]any{}
	if err := endpointHysteria(p, "sing-box", out); err != nil || out["hop_interval"] != "30s" || !endpointHysteriaTimingOmitted(p, "sing-box") {
		t.Fatal("invalid optional timing broke endpoint", out, err)
	}
	p.Protocol = "vless"
	if err := endpointHysteria(p, "xray", out); err != nil || hasEndpointHysteriaPorts(p) {
		t.Fatal("retired protocol option affected endpoint", err)
	}
}

func TestEndpointHysteriaDoesNotSilentlyChangeInheritedMaskSemantics(t *testing.T) {
	for _, settings := range []map[string]any{{"mode": "intervalLocal", "interval": 30}, {"mode": "intervalRemote", "remotePorts": "443", "remoteIPs": []any{"192.0.2.1"}}} {
		p := Proxy{Protocol: "hysteria", Stream: map[string]any{"finalmask": map[string]any{"udp": []any{map[string]any{"type": "udphop", "settings": settings}}}}}
		if err := endpointHysteria(p, "xray", map[string]any{"streamSettings": p.Stream}); err != nil {
			t.Fatal("native mask was unnecessarily changed", err)
		}
		if err := endpointHysteria(p, "clash", map[string]any{}); !errors.Is(err, ErrUnsupported) {
			t.Fatal("unrepresentable required transport silently dropped", err)
		}
		ports := []TuningPortRange{{Min: 443, Max: 444}}
		p.EndpointTuning = &EndpointOptions{HysteriaPorts: &ports}
		if err := endpointHysteria(p, "xray", map[string]any{}); !errors.Is(err, ErrUnsupported) {
			t.Fatal("explicit edit silently ignored opaque inherited mask", err)
		}
	}
}
