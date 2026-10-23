package subscription

import "testing"

func TestSplitNamesCombineSharedTermsAndCanonicalIPFamilies(t *testing.T) {
	for _, tc := range []struct{ up, down, want string }{
		{"【1.0x】Hinet家宽直连-V6", "【1.0x】Hinet家宽直连-V4", "【1.0x】Hinet家宽直连-V4V6"},
		{"Hinet-V4", "Hinet-V6", "Hinet-V4V6"},
		{"Hinet-IPv6-direct", "Hinet-IPv4-direct", "Hinet-IPv4IPv6-direct"},
		{"Hinet-route-A", "Hinet-route-B", "Hinet-route-A / B"},
		{"same", "same", "same"},
		{"Tokyo", "Singapore", "Tokyo ↑ / Singapore ↓"},
	} {
		if got := splitProfileName(tc.up, tc.down); got != tc.want {
			t.Errorf("%s + %s = %s, want %s", tc.up, tc.down, got, tc.want)
		}
	}
}
