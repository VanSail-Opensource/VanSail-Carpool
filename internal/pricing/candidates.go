package pricing

import (
	"math"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

// NumericalCandidates contains no wallet, credentials, owner or product identity.
// Authorization and all economic safety gates remain on the master.
func NumericalCandidates(in OptimizationInputs) []compute.CandidateInput {
	rows := make([]compute.CandidateInput, 0, 101)
	seen := map[int64]bool{}
	for step := int64(0); step <= 100; step++ {
		price := min(MaxPrice, in.Policy.FixedPrice*2*step/100)
		if in.Policy.ProductType == "payg" {
			price = max(1, price)
		}
		if seen[price] {
			continue
		}
		seen[price] = true
		rows = append(rows, compute.CandidateInput{Price: price, NetPrice: price, Units: in.SalesUnits, Cost: in.CostMinor, Features: ResponseObservation{LogPrice: math.Log(float64(max(1, price)) / float64(max(1, in.Policy.FixedPrice)))}})
	}
	return rows
}

func SelectCandidates(in OptimizationInputs, values []compute.Candidate) (PriceCandidate, []PriceCandidate, error) {
	best := PriceCandidate{Price: in.Current, Contribution: math.Inf(-1)}
	curve := make([]PriceCandidate, 0, len(values))
	safe := in.CapacityGB > in.CommittedGB && in.DemandGB >= 0 && in.DemandGB <= in.CapacityGB-in.CommittedGB && in.CostMinor >= 0
	for _, c := range values {
		p := PriceCandidate{Price: c.Price, Probability: c.Probability, Revenue: c.Revenue, Contribution: c.Contribution, Eligible: safe && in.Model.Validated && in.ForecastValidated && c.Revenue >= in.CostMinor}
		curve = append(curve, p)
		if p.Eligible && p.Contribution > best.Contribution {
			best = p
		}
	}
	if !safe {
		return PriceCandidate{Price: in.Current}, curve, CapacityUnsafe
	}
	if math.IsInf(best.Contribution, -1) {
		return PriceCandidate{Price: in.Current}, curve, ModelUnvalidated
	}
	return best, curve, nil
}

func CandidateLimits(reference int64, best PriceCandidate, curve []PriceCandidate) (int64, int64) {
	if reference <= 0 {
		return 0, 0
	}
	low, high := best.Price, best.Price
	for _, c := range curve {
		if c.Eligible && c.Contribution >= best.Contribution*.95 {
			low, high = min(low, c.Price), max(high, c.Price)
		}
	}
	return max(0, (high-reference)*Scale/reference), max(0, (reference-low)*Scale/reference)
}
