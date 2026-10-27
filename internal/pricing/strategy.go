package pricing

import "math"

// StrategyVersion separates revised economic evidence from historical decisions.
const StrategyVersion = "balanced-coupons-v4"

// PriceSupport retains the observed gross-price range, separately for coupon owners.
// It is analytical lineage, never permission to consume a coupon or debit a wallet.
type PriceSupport struct {
	Samples       int     `json:"samples"`
	Low           float64 `json:"lowLogPrice"`
	High          float64 `json:"highLogPrice"`
	CouponSamples int     `json:"couponSamples"`
	CouponLow     float64 `json:"couponLowLogPrice"`
	CouponHigh    float64 `json:"couponHighLogPrice"`
}

func ObservedSupport(rows []ResponseObservation) PriceSupport {
	s := PriceSupport{}
	for _, r := range rows {
		if s.Samples == 0 {
			s.Low, s.High = r.LogPrice, r.LogPrice
		}
		s.Samples++
		s.Low, s.High = min(s.Low, r.LogPrice), max(s.High, r.LogPrice)
		if r.CouponPresent > 0 && r.CouponDiscount > 0 {
			if s.CouponSamples == 0 {
				s.CouponLow, s.CouponHigh = r.LogPrice, r.LogPrice
			}
			s.CouponSamples++
			s.CouponLow, s.CouponHigh = min(s.CouponLow, r.LogPrice), max(s.CouponHigh, r.LogPrice)
		}
	}
	return s
}

func (s PriceSupport) Allows(price, reference int64, coupon bool) bool {
	low, high, samples := s.Low, s.High, s.Samples
	minimum := 200
	if coupon {
		low, high, samples, minimum = s.CouponLow, s.CouponHigh, s.CouponSamples, 30
	}
	x := math.Log(float64(max(1, price)) / float64(max(1, reference)))
	return samples >= minimum && high-low > .1 && x >= low-1e-12 && x <= high+1e-12
}

// StableCandidate chooses the least monetary change within the positive 95% envelope.
// Retention, then the lower gross price, resolve ties deterministically.
func StableCandidate(current int64, curve []PriceCandidate) (PriceCandidate, bool) {
	peak := 0.0
	for _, c := range curve {
		if c.Eligible {
			peak = max(peak, c.Contribution)
		}
	}
	if peak <= 0 {
		return PriceCandidate{Price: current}, false
	}
	var best PriceCandidate
	found := false
	for _, c := range curve {
		if !c.Eligible || c.Contribution < peak*.95 {
			continue
		}
		move, previous := math.Abs(float64(c.Price-current)), math.Abs(float64(best.Price-current))
		if !found || move < previous || move == previous && (c.Probability > best.Probability || c.Probability == best.Probability && c.Price < best.Price) {
			best, found = c, true
		}
	}
	return best, found
}
