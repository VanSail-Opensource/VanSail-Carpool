package xnode

import "testing"

func TestBlockedPortRangesStayCompactAndRejectInvalidInput(t *testing.T) {
	ranges, err := ParseBlockedPorts("443,1-100,80-120,121,65535")
	if err != nil || len(ranges) != 3 || ranges[0].From != 1 || ranges[0].To != 121 {
		t.Fatalf("ranges=%+v,error=%v", ranges, err)
	}
	for _, raw := range []string{"0", "65536", "100-1", "1,,2", "1-2-3", "-1", "x"} {
		if _, err = ParseBlockedPorts(raw); err == nil {
			t.Fatalf("invalid ports accepted: %s", raw)
		}
	}
}
