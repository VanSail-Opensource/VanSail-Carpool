package pricing

// OptimalLimits derives a near-optimal price envelope from the same expected
// contribution objective as publication. It never treats a prior as price evidence.
// The bounded 101-point search covers 0..2x reference; prices retaining at least
// 95% of the best nonnegative contribution define the automatic constraints.
func OptimalLimits(in OptimizationInputs) (increase, discount int64) {
	if !in.Model.Validated || !in.ForecastValidated || in.Policy.FixedPrice <= 0 {
		return 0, 0
	}
	probe := in
	probe.Policy.MaxIncreaseBPS, probe.Policy.MaxDiscountBPS = Scale, Scale
	probe.DiscountAuthorized = true
	best, curve, err := Optimize(probe)
	if err != nil || best.Contribution <= 0 {
		return 0, 0
	}
	return CandidateLimits(in.Policy.FixedPrice, best, curve)
}
