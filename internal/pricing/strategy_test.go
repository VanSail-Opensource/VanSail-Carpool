package pricing

import (
	"context"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	"math"
	"testing"
)

func TestBalancedCandidatesWeightCostOnceAndPreferStability(t *testing.T) {
	in := OptimizationInputs{Policy: DefaultPolicy("payg", "s", 1000), Current: 1000, SalesUnits: 1, CostMinor: 100, CapacityGB: 10, DemandGB: 1, ForecastValidated: true, Model: ResponseModel{Validated: true}}
	inputs := NumericalCandidates(in)
	for i := range inputs {
		inputs[i].Price = max(1, inputs[i].Price)
	}
	task := compute.Task{ID: "economics", Kind: "candidate_eval_v1", Model: in.Model, Candidates: inputs}
	out, err := compute.Run(context.Background(), compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range out.Items[0].Candidates {
		want := .5 * (float64(c.Price) - 100)
		if math.Abs(c.Contribution-want) > 1e-9 {
			t.Fatalf("contribution %v, want %v", c.Contribution, want)
		}
	}
	curve := []PriceCandidate{{Price: 1000, Probability: .6, Contribution: 96, Eligible: true}, {Price: 1100, Probability: .5, Contribution: 100, Eligible: true}, {Price: 900, Probability: .7, Contribution: 94, Eligible: true}}
	got, ok := StableCandidate(1000, curve)
	if !ok || got.Price != 1000 {
		t.Fatal(got, ok)
	}
	curve[0].Contribution = 94
	got, ok = StableCandidate(1000, curve)
	if !ok || got.Price != 1100 {
		t.Fatal(got, ok)
	}
}
func TestPriceSupportRefusesExtrapolationAndWeakCouponCohorts(t *testing.T) {
	s := PriceSupport{Samples: 200, Low: math.Log(.8), High: math.Log(1.2), CouponSamples: 30, CouponLow: math.Log(.9), CouponHigh: math.Log(1.1)}
	for _, tc := range []struct {
		price        int64
		coupon, want bool
	}{{1000, false, true}, {1300, false, false}, {1000, true, true}, {1200, true, false}} {
		if got := s.Allows(tc.price, 1000, tc.coupon); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	s.CouponSamples = 29
	if s.Allows(1000, 1000, true) {
		t.Fatal("weak coupon cohort accepted")
	}
}

func TestCandidateGridIncludesExactCurrentPrice(t *testing.T) {
	in := OptimizationInputs{Policy: DefaultPolicy("combo", "c", 1000), Current: 1003}
	found := false
	for _, candidate := range NumericalCandidates(in) {
		found = found || candidate.Price == 1003
	}
	if !found {
		t.Fatal("candidate grid would force an unnecessary change")
	}
}
