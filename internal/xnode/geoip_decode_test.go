package xnode

import (
	"bytes"
	"context"
	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"net/netip"
	"testing"
)

func TestCountryPrefixesRemainCompactAndValidateCoverage(t *testing.T) {
	data := &geodata.GeoIPList{Entry: []*geodata.GeoIP{{Code: "CN", Cidr: []*geodata.CIDR{{Ip: netip.MustParseAddr("1.1.1.0").AsSlice(), Prefix: 25}, {Ip: netip.MustParseAddr("1.1.1.128").AsSlice(), Prefix: 25}}}, {Code: "US", Cidr: []*geodata.CIDR{{Ip: netip.MustParseAddr("8.8.8.0").AsSlice(), Prefix: 24}}}}}
	raw, err := proto.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	prefixes, err := readGeoIPPrefixes(context.Background(), bytes.NewReader(raw), []string{"geoip:cn"})
	if err != nil || len(prefixes) != 1 || prefixes[0] != "1.1.1.0/24" {
		t.Fatalf("uncompressed or wrong country prefixes: %v %v", prefixes, err)
	}
	if _, err = readGeoIPPrefixes(context.Background(), bytes.NewReader(raw), []string{"geoip:cn", "geoip:gb"}); err == nil {
		t.Fatal("absent country silently accepted")
	}
	if _, err = readGeoIPPrefixes(context.Background(), bytes.NewReader(raw[:len(raw)-1]), []string{"geoip:cn"}); err == nil {
		t.Fatal("truncated asset accepted")
	}
	data.Entry[0].ReverseMatch = true
	raw, _ = proto.Marshal(data)
	if _, err = readGeoIPPrefixes(context.Background(), bytes.NewReader(raw), []string{"geoip:cn"}); err == nil {
		t.Fatal("inverse country policy was silently widened")
	}
}

func TestLargeUnselectedCountryDoesNotPreventSavingCountryFilter(t *testing.T) {
	entry := protowire.AppendTag(nil, 1, protowire.BytesType)
	entry = protowire.AppendString(entry, "US")
	entry = protowire.AppendTag(entry, 99, protowire.BytesType)
	entry = protowire.AppendBytes(entry, make([]byte, 17<<20))
	raw := protowire.AppendTag(nil, 1, protowire.BytesType)
	raw = protowire.AppendBytes(raw, entry)
	data, err := proto.Marshal(&geodata.GeoIPList{Entry: []*geodata.GeoIP{{Code: "CN", Cidr: []*geodata.CIDR{{Ip: []byte{1, 1, 1, 0}, Prefix: 24}}}}})
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, data...)
	got, err := readGeoIPPrefixes(context.Background(), bytes.NewReader(raw), []string{"geoip:cn"})
	if err != nil || len(got) != 1 || got[0] != "1.1.1.0/24" {
		t.Fatalf("country projection = %v, %v", got, err)
	}
}
