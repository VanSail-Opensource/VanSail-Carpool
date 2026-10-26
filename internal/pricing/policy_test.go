package pricing

import "testing"

func TestPolicyDefaultsAndExactBounds(t *testing.T) {
	p := DefaultPolicy("combo", "standard", 1000)
	if p.DynamicEnabled || p.PurchaseMode != "FIXED" || p.RenewalMode != "FIXED" {
		t.Fatal("product activated implicitly")
	}
	for _, tc := range []struct {
		name                                          string
		reference, proposed, increase, discount, want int64
	}{
		{"increase", 1000, 2000, 500, 0, 1050}, {"discount", 1000, 1, 0, 500, 950},
		{"round_down", 101, 1, 0, 500, 95}, {"max", MaxPrice, MaxPrice, 10000, 10000, MaxPrice},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BoundPrice(tc.reference, tc.proposed, tc.increase, tc.discount)
			if err != nil || got != tc.want {
				t.Fatalf("got %d %v", got, err)
			}
		})
	}
	if _, err := BoundPrice(MaxPrice+1, 100, 0, 0); err == nil {
		t.Fatal("overflow input accepted")
	}
	p.PurchaseMode = "DYNAMIC"
	if p.Validate() == nil {
		t.Fatal("implicit activation accepted")
	}
}
