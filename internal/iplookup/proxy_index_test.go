package iplookup

import (
	"net/netip"
	"testing"
)

func testPrefixIndex(values ...string) *prefixIndex {
	builder := newPrefixIndexBuilder(0)
	for _, value := range values {
		builder.add(netip.MustParsePrefix(value))
	}
	return builder.freeze()
}

func TestCompressedPrefixMembershipAndLimits(t *testing.T) {
	b := newPrefixIndexBuilder(5)
	for _, raw := range []string{"11.0.0.0/24", "11.0.0.7/32", "11.0.0.7/32", "2001:ac8:16::/48", "2001:ac8:16::1/128", "2001:ac8:17::/48"} {
		if !b.add(netip.MustParsePrefix(raw)) {
			t.Fatal("duplicate consumed entry allowance")
		}
	}
	if b.add(netip.MustParsePrefix("12.0.0.0/24")) {
		t.Fatal("new prefix exceeded limit")
	}
	index := b.freeze()
	if index.count() != 5 {
		t.Fatalf("count=%d", index.count())
	}
	for _, tc := range []struct {
		ip   string
		want bool
	}{
		{"11.0.0.0", true}, {"11.0.0.255", true}, {"11.0.1.0", false},
		{"::ffff:11.0.0.7", true}, {"2001:ac8:16:ffff::1", true},
		{"2001:ac8:17::1", true}, {"2001:ac8:18::1", false},
	} {
		t.Run(tc.ip, func(t *testing.T) {
			if got := index.contains(netip.MustParseAddr(tc.ip)); got != tc.want {
				t.Fatalf("contains=%v", got)
			}
		})
	}
	if index.contains(netip.Addr{}) {
		t.Fatal("invalid address matched")
	}
}

func TestCompressedUnionPreservesInputsAndPreviousAtLimit(t *testing.T) {
	makeIndex := func(values ...string) *prefixIndex {
		builder := newPrefixIndexBuilder(0)
		for _, value := range values {
			builder.add(netip.MustParsePrefix(value))
		}
		return builder.freeze()
	}
	previous := makeIndex("11.0.0.8/32", "2001:ac8:16::1/128")
	incoming := makeIndex("11.0.0.1/32", "11.0.0.8/32", "2001:ac8:16::2/128")
	limited := mergePrefixIndexes(previous, incoming, 3)
	if limited.count() != 3 || !limited.contains(netip.MustParseAddr("2001:ac8:16::1")) || !limited.contains(netip.MustParseAddr("11.0.0.8")) {
		t.Fatal("lost previous evidence")
	}
	combined := mergePrefixIndexes(previous, incoming, 0)
	if combined.count() != 4 || previous.count() != 2 || incoming.count() != 3 {
		t.Fatal("union mutated inputs or miscounted duplicate")
	}
	if mergePrefixIndexes(previous, nil, 3) != previous {
		t.Fatal("failed download copied published evidence")
	}
}
