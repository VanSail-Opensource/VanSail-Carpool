package pricing

import (
	"math"
)

// OptimizationInputs keeps cost recovery, capacity and statistical gates separate.
type OptimizationInputs struct {
	Policy             Policy
	Current            int64
	DemandGB           float64
	SalesUnits         float64
	CostMinor          float64
	CapacityGB         float64
	CommittedGB        float64
	Model              ResponseModel
	ForecastValidated  bool
	DiscountAuthorized bool
	Support            *PriceSupport
}

type PriceCandidate struct {
	Price        int64   `json:"priceMinor,string"`
	Probability  float64 `json:"probability"`
	Revenue      float64 `json:"revenueMinor"`
	Contribution float64 `json:"contributionMinor"`
	Eligible     bool    `json:"eligible"`
}

// Optimize produces an explainable bounded candidate curve. Capacity pressure
// rejects new admissions; it is never repaired by inventing a scarcity price.
func Optimize(in OptimizationInputs) (PriceCandidate, []PriceCandidate, error) {
	if in.DemandGB < 0 || in.CostMinor < 0 || in.CapacityGB <= in.CommittedGB || in.DemandGB > in.CapacityGB-in.CommittedGB {
		return PriceCandidate{}, nil, CapacityUnsafe
	}
	best := PriceCandidate{Price: in.Current, Contribution: math.Inf(-1)}
	out := []PriceCandidate{}
	seen := map[int64]bool{}
	for step := 0; step <= 100; step++ {
		discount := int64(0)
		if in.DiscountAuthorized {
			discount = in.Policy.MaxDiscountBPS
		}
		low := in.Policy.FixedPrice * (Scale - discount) / Scale
		high := min(MaxPrice, in.Policy.FixedPrice+in.Policy.FixedPrice*in.Policy.MaxIncreaseBPS/Scale)
		proposed := low + (high-low)*int64(step)/100
		price, err := BoundPrice(in.Policy.FixedPrice, max(0, proposed), in.Policy.MaxIncreaseBPS, discount)
		if err != nil {
			return best, nil, err
		}
		if seen[price] {
			continue
		}
		seen[price] = true
		logPrice := math.Log(float64(max(1, price)) / float64(max(1, in.Policy.FixedPrice)))
		prob := in.Model.Predict(ResponseObservation{LogPrice: logPrice})
		revenue := float64(price) * in.SalesUnits * prob
		contribution := prob * (float64(price)*in.SalesUnits - in.CostMinor)
		eligible := contribution >= 0 && in.Model.Validated && in.ForecastValidated && (in.Support == nil || in.Support.Allows(price, in.Policy.FixedPrice, false))
		c := PriceCandidate{Price: price, Probability: prob, Revenue: revenue, Contribution: contribution, Eligible: eligible}
		out = append(out, c)
		if eligible && c.Contribution > best.Contribution {
			best = c
		}
	}
	if math.IsInf(best.Contribution, -1) {
		return best, out, ModelUnvalidated
	}
	if stable, ok := StableCandidate(in.Current, out); ok {
		best = stable
	} else {
		return PriceCandidate{Price: in.Current}, out, ModelUnvalidated
	}
	return best, out, nil
}
