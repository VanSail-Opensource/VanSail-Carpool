package xnode

import (
	"bytes"
	"context"
	"testing"

	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"
)

func TestGeoSiteLookupRetainsNativeParsingAndAttributeSemantics(t *testing.T) {
	for _, test := range []struct {
		name, code, attribute, selector, expression string
		valid                                       bool
	}{
		{"normalized selector", "GOOGLE", "ai", "geosite:Google@AI", "google.com", true},
		{"case sensitive asset attribute", "GOOGLE", "AI", "geosite:google@ai", "google.com", false},
		{"case sensitive asset category", "google", "ai", "geosite:google@ai", "google.com", false},
		{"empty attribute", "GOOGLE", "", "geosite:google@", "google.com", false},
		{"invalid embedded regexp", "GOOGLE", "ai", "geosite:google", "[", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := proto.Marshal(&geodata.GeoSiteList{Entry: []*geodata.GeoSite{{Code: test.code, Domain: []*geodata.Domain{{Type: geodata.Domain_Regex, Value: test.expression, Attribute: []*geodata.Domain_Attribute{{Key: test.attribute}}}}}}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = readRoutingGeoSite(context.Background(), bytes.NewReader(raw), []string{test.selector})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v: %v", test.valid, err)
			}
		})
	}
}
