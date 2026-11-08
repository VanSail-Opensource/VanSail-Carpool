package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
	"testing"
)

func TestRenewalsUsePublicPriceWithinPurchasedBounds(t *testing.T) {
	s := &Store{}
	for _, test := range []struct {
		name    string
		public  int64
		enabled bool
		want    int64
	}{
		{"public increase capped", 1500, true, 1100},
		{"public decrease", 900, true, 900},
		{"fixed or unavailable", 1500, false, 1000},
	} {
		t.Run(test.name, func(t *testing.T) {
			in := renewalPriceSnapshot{Contract: renewal.Contract{Price: 1000, Ceiling: 1100, MaxChangeBPS: 1000}, Public: test.public, PublicEnabled: test.enabled}
			candidate, curve, err := s.calculateRenewalPrice(context.Background(), in)
			if err != nil || candidate.Price != test.want || len(curve) != 1 {
				t.Fatal(candidate, curve, err)
			}
		})
	}
}
