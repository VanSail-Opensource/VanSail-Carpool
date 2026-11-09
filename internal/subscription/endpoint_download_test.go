package subscription

import (
	"encoding/json"
	"testing"

	xrayconf "github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/splithttp"
)

func TestEndpointPartialDownloadInheritsNativeDestinationAndSession(t *testing.T) {
	p := endpointReviewProxy()
	p.EndpointTuning = &EndpointOptions{}
	object(p.Stream, "xhttpSettings")["extra"] = map[string]any{
		"sessionIDPlacement": "header", "sessionIDKey": "X-Session-ID",
		"headers": map[string]any{"Main": "inherited"}, "xmux": map[string]any{"maxConnections": 2},
		"downloadSettings": map[string]any{"address": "down.example", "xhttpSettings": map[string]any{
			"path": "/download", "headers": map[string]any{"Ignored-Flat": "wrong"},
			"extra": map[string]any{"headers": map[string]any{"Download": "explicit"}, "xmux": map[string]any{"hMaxRequestTimes": 300}},
		}},
	}
	before, _ := json.Marshal(p.Stream)
	out, err := p.Outbound()
	if err != nil {
		t.Fatal(err)
	}
	stream := object(out, "streamSettings")
	down := object(effectiveXHTTP(object(stream, "xhttpSettings")), "downloadSettings")
	settings := object(down, "xhttpSettings")
	extra := effectiveXHTTP(settings)
	if text(down, "address") != "down.example" || text(settings, "path") != "/download" || text(extra, "sessionIDKey") != "X-Session-ID" || object(extra, "xmux")["maxConnections"] != float64(2) || object(extra, "xmux")["hMaxRequestTimes"] != float64(300) {
		t.Fatal("partial download lost inherited or explicit settings", down)
	}
	headers := object(extra, "headers")
	if headers["Main"] != "inherited" || headers["Download"] != "explicit" || headers["Ignored-Flat"] != nil || extra["downloadSettings"] != nil {
		t.Fatal("partial extra merge resurrected ignored data or recursed", down)
	}
	raw, err := json.Marshal(stream)
	if err != nil {
		t.Fatal(err)
	}
	var source xrayconf.StreamConfig
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	built, err := source.Build()
	if err != nil {
		t.Fatal(err)
	}
	memory, err := internet.ToMemoryStreamConfig(built)
	if err != nil {
		t.Fatal(err)
	}
	download, err := internet.ToMemoryStreamConfig(memory.ProtocolSettings.(*splithttp.Config).DownloadSettings)
	if err != nil || download.Destination == nil || download.ProtocolSettings.(*splithttp.Config).Path != "/download" {
		t.Fatal("pinned dialer would lack destination or XHTTP stream", download, err)
	}
	after, _ := json.Marshal(p.Stream)
	if string(before) != string(after) {
		t.Fatal("download defaults mutated endpoint state")
	}
}

func TestEndpointWrongDownloadTransportExcludesOnlyAffectedEndpoint(t *testing.T) {
	p, peer := endpointReviewProxy(), endpointReviewProxy()
	peer.ID = "safe-peer"
	p.EndpointTuning, peer.EndpointTuning = &EndpointOptions{}, &EndpointOptions{}
	object(p.Stream, "xhttpSettings")["extra"] = map[string]any{"downloadSettings": map[string]any{"network": "grpc", "address": "down.example", "port": 443}}
	selected, excluded, err := CompatibleProxies("xray", []Proxy{p, peer})
	if err != nil || len(selected) != 1 || selected[0].ID != peer.ID || excluded != 1 {
		t.Fatal("invalid independent download stream affected compatible peer", selected, excluded, err)
	}
	if failures := ValidateEndpointOptions(EndpointOptions{}, p); failures["downloadEnabled"] != "transport" {
		t.Fatal("invalid independent download stream was accepted", failures)
	}
}
