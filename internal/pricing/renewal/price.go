package renewal

import (
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// Candidate exposes utility components without treating propensity as discount permission.
type Candidate struct {
	Price        int64   `json:"priceMinor,string"`
	Probability  float64 `json:"probability"`
	Contribution float64 `json:"contribution"`
	Utility      float64 `json:"utility"`
	Eligible     bool    `json:"eligible"`
}

// Inputs distinguishes economic evidence, policy eligibility and statistical confidence.
type Inputs struct {
	Contract           Contract
	PublicPrice        int64
	Cost               float64
	FutureContribution float64
	BaseProbability    float64
	Elasticity         float64
	DiscountEligible   bool
	DiscountBPS        int64
	Validated          bool
	Response           *pricing.ResponseModel
	Activity           float64
	PriorRenewals      float64
}

// Recommend considers current, public, ceiling and authorized loyalty prices.
// Prior-only models remain shadow recommendations and never invent price elasticity.
func Recommend(in Inputs) (Candidate, []Candidate, error) {
	if err := in.Contract.Validate(); err != nil {
		return Candidate{}, nil, err
	}
	prices := []int64{in.Contract.Price, in.PublicPrice, in.Contract.Ceiling}
	if in.DiscountEligible && in.DiscountBPS > 0 {
		prices = append(prices, in.Contract.Price*(10000-min(in.DiscountBPS, 10000))/10000)
	}
	best := Candidate{Price: in.Contract.Price, Utility: math.Inf(-1)}
	out := make([]Candidate, 0, len(prices))
	seen := map[int64]bool{}
	for _, p := range prices {
		if seen[p] {
			continue
		}
		seen[p] = true
		eligible := p >= 0 && p <= in.Contract.Ceiling && p <= in.Contract.Price+in.Contract.Price*in.Contract.MaxChangeBPS/10000
		addonBasis := int64(0)
		for _, a := range in.Contract.Addons {
			addonBasis += a.Price
		}
		if p < addonBasis {
			eligible = false
		}
		if p < in.Contract.Price && p != in.PublicPrice && (!in.DiscountEligible || p < in.Contract.Price*(10000-in.DiscountBPS)/10000) {
			eligible = false
		}
		if !in.Validated && p != in.Contract.Price {
			eligible = false
		}
		probability := pricing.Probability(in.BaseProbability)
		if in.Validated && in.Contract.Price > 0 {
			probability = pricing.Probability(probability * math.Exp(-max(0, in.Elasticity)*float64(p-in.Contract.Price)/float64(in.Contract.Price)))
			if in.Response != nil {
				probability = in.Response.Predict(pricing.ResponseObservation{LogPrice: math.Log(float64(max(1, p)) / float64(in.Contract.Price)), Activity: in.Activity, Renewals: in.PriorRenewals})
			}
		}
		contribution := float64(p) - max(0, in.Cost)
		volatility := math.Abs(float64(p-in.Contract.Price)) * 0.05
		c := Candidate{Price: p, Probability: probability, Contribution: contribution, Utility: probability*(contribution+in.FutureContribution) - volatility, Eligible: eligible && contribution >= 0}
		out = append(out, c)
		if c.Eligible && c.Utility > best.Utility {
			best = c
		}
	}
	if math.IsInf(best.Utility, -1) {
		return best, out, pricing.RenewalQuoteUnavailable
	}
	return best, out, nil
}
