package xnode

import (
	"testing"
	"time"
)

func TestFirewallNestedFilter(t *testing.T) {
	filter := FirewallFilter{Op: "and", Children: []FirewallFilter{
		{Op: "or", Children: []FirewallFilter{{Field: "rule", Value: "xray_tcp_probe"}, {Field: "rule", Value: "xray_auth_failure"}}},
		{Field: "status", Value: "reported"}, {Field: "hits", Value: "2"},
		{Field: "ip", Value: "2001:4860::/32"},
	}}
	predicate, err := CompileFirewallFilter(filter)
	if err != nil {
		t.Fatal(err)
	}
	record := FirewallRecord{Prefix: "2001:4860:1234:5678::/64", Rule: "xray_auth_failure", Count: "3", DetectedAt: time.Now()}
	if !predicate(record) {
		t.Fatal("nested expression rejected matching record")
	}
	record.Banned = true
	if predicate(record) {
		t.Fatal("nested expression accepted a banned record")
	}
	filter.Children = append(filter.Children, FirewallFilter{Field: "detectedAt", From: "invalid"})
	if _, err = CompileFirewallFilter(filter); err == nil {
		t.Fatal("invalid timestamp accepted")
	}
}

func TestFirewallFilterLimits(t *testing.T) {
	filter := FirewallFilter{Op: "and"}
	for range 33 {
		filter.Children = append(filter.Children, FirewallFilter{Field: "hits", Value: "1"})
	}
	if _, err := CompileFirewallFilter(filter); err == nil {
		t.Fatal("too many conditions accepted")
	}
	deep := FirewallFilter{Op: "and", Children: []FirewallFilter{{Op: "or", Children: []FirewallFilter{{Op: "and", Children: []FirewallFilter{{Op: "or", Children: []FirewallFilter{{Field: "status", Value: "banned"}}}}}}}}}
	if _, err := CompileFirewallFilter(deep); err == nil {
		t.Fatal("excessive nesting accepted")
	}
}
