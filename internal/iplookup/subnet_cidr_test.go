package iplookup

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSubnetSkipsIPv6WithoutCollectingEvidence(t *testing.T) {
	for _, ip := range []string{"2001:ac8:16::1234", "2606:4700:4700::1111"} {
		t.Run(ip, func(t *testing.T) {
			s := NewEnhancedService(nil, nil, nil)
			s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) {
				t.Error("IPv6 subnet scan collected proxy sources")
				return nil, errors.New("unexpected collection")
			}
			block := AbuseBlock{LiveMeta: LiveMeta{Status: "success"}, Prefix: blockPrefix(ip)}
			r := Report{IP: ip, LiveDetails: &LiveDetails{Block: block}, Subnet: &SubnetResult{Status: "processing", Addresses: []SubnetAddress{{IP: ip}}}}
			saved := 0
			if err := s.checkSubnet(context.Background(), &r, DefaultConfig(), func() error { saved++; return nil }); err != nil {
				t.Fatal(err)
			}
			if saved != 1 || r.Subnet.Status != "skipped" || r.Subnet.Reason != "ipv6" || r.Subnet.Prefix != blockPrefix(ip) || r.Subnet.Addresses == nil || len(r.Subnet.Addresses) != 0 {
				t.Fatalf("IPv6 scan was not skipped: %+v saves=%d", r.Subnet, saved)
			}
			if !reflect.DeepEqual(r.LiveDetails.Block, block) {
				t.Fatal("subnet exclusion changed independently collected live evidence")
			}
			// With no live service or prior evidence, exclusion must still be local.
			r.LiveDetails, r.Subnet = nil, nil
			if err := s.checkSubnet(context.Background(), &r, DefaultConfig(), func() error { return nil }); err != nil || r.Subnet.Status != "skipped" {
				t.Fatalf("IPv6 exclusion required upstream evidence: %v", err)
			}
			failure := errors.New("save unavailable")
			if err := s.checkSubnet(context.Background(), &r, DefaultConfig(), func() error { return failure }); !errors.Is(err, failure) {
				t.Fatalf("lost persistence error: %v", err)
			}
		})
	}
}

func TestSubnetRejectsInvalidOrScopedAddressesBeforeCollection(t *testing.T) {
	for _, ip := range []string{"invalid", "fe80::1%eth0"} {
		t.Run(ip, func(t *testing.T) {
			s := NewEnhancedService(nil, nil, nil)
			r := Report{IP: ip}
			saved := false
			err := s.checkSubnet(context.Background(), &r, DefaultConfig(), func() error { saved = true; return nil })
			if ErrorCode(err) != "IP_LOOKUP_INVALID_IP" || saved || r.Subnet != nil {
				t.Fatalf("invalid address entered subnet work: %v saved=%v", err, saved)
			}
		})
	}
}

func TestSubnetCIDRMembershipSkipsAddressProviders(t *testing.T) {
	s := NewEnhancedService(nil, nil, nil)
	c := DefaultConfig()
	s.pools[proxyPoolKey(c)] = &proxyPool{
		indexes: []*prefixIndex{testPrefixIndex("102.129.155.0/24")}, sources: []ProxySourceResult{{ID: "ranges", Name: "Ranges", Category: ProxyPoolCategory}},
		complete: true,
		expires:  time.Now().Add(time.Minute),
	}
	r := NewReport("subnet-cidr", "102.129.155.42", c)
	if err := s.checkSubnet(context.Background(), &r, c, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if r.Subnet.Status != "success" || len(r.Subnet.Addresses) != 256 {
		t.Fatalf("unexpected subnet coverage: %+v", r.Subnet)
	}
	for _, row := range r.Subnet.Addresses {
		if row.Status != "flagged" || row.Proxy != "flagged" || row.Abuse != "skipped" || row.IPAPI != "skipped" || !reflect.DeepEqual(row.Sources, []string{"ranges"}) {
			t.Fatalf("CIDR did not short-circuit address providers: %+v", row)
		}
	}
}
