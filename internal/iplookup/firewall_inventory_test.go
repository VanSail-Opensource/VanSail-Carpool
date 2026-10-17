package iplookup

import (
	"context"
	"net/netip"
	"testing"
)

func TestFirewallExportKeepsCompactCIDRs(t *testing.T) {
	builder := newPrefixIndexBuilder(100)
	expected := map[string]bool{"8.0.0.0/8": true, "2001:4860::/32": true, "1.1.1.1/32": true}
	for raw := range expected {
		builder.add(netip.MustParsePrefix(raw))
	}
	total := 0
	if err := builder.freeze().walkPrefixes(context.Background(), func(page []string) error {
		for _, raw := range page {
			if !expected[raw] {
				t.Errorf("unexpected expanded entry %s", raw)
			}
			total++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if total != len(expected) {
		t.Fatal("compact ranges were expanded or omitted")
	}
}
