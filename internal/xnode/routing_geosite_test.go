package xnode

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"
)

func TestRoutingGeoSiteStreamingUsesPinnedDomainKindsAndAllAttributes(t *testing.T) {
	list := &geodata.GeoSiteList{Entry: []*geodata.GeoSite{{Code: "GOOGLE", Domain: []*geodata.Domain{
		{Type: geodata.Domain_Domain, Value: "google.com"},
		{Type: geodata.Domain_Full, Value: "ai.google.com", Attribute: []*geodata.Domain_Attribute{{Key: "AI"}, {Key: "US"}}},
		{Type: geodata.Domain_Regex, Value: `^google[0-9]+\.example$`, Attribute: []*geodata.Domain_Attribute{{Key: "US"}}},
	}}}}
	raw, err := proto.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := readRoutingGeoSite(context.Background(), bytes.NewReader(raw), []string{"geosite:google", "geosite:google@ai@us"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected["geosite:google@ai@us"], []string{"full:ai.google.com"}) || len(selected["geosite:google"]) != 3 {
		t.Fatalf("attribute or kind mismatch: %v", selected)
	}
	if _, err = readRoutingGeoSite(context.Background(), bytes.NewReader(raw), []string{"geosite:missing"}); err == nil {
		t.Fatal("missing country silently ignored")
	}
	if _, err = readRoutingGeoSite(context.Background(), bytes.NewReader(raw[:len(raw)-1]), []string{"geosite:google"}); err == nil {
		t.Fatal("truncated asset accepted")
	}
}
func TestRoutingContextRejectsUntrackedHopsAndDifferentDetection(t *testing.T) {
	root := map[string]any{"outbounds": []any{map[string]any{"tag": "direct", "protocol": "freedom"}}, "routing": map[string]any{"rules": []any{map[string]any{"UserRouting": map[string]any{}}}}, "inbounds": []any{map[string]any{"tag": "one"}, map[string]any{"tag": "two", "sniffing": map[string]any{"enabled": true}}}}
	if _, err := routingServerContext(root, []string{"one"}, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := routingServerContext(root, []string{"one", "two"}, false, false); err == nil {
		t.Fatal("different destination detection accepted")
	}
	root["outbounds"] = []any{map[string]any{"tag": "untracked", "protocol": "vless"}}
	if _, err := routingServerContext(root, []string{"one"}, false, false); err == nil {
		t.Fatal("untracked proxy fallback accepted")
	}
}
