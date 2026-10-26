package database

import (
	"context"
	"errors"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func (s *Store) calculateRenewalPrice(ctx context.Context, in renewalPriceSnapshot) (renewal.Candidate, []renewal.Candidate, error) {
	c := in.Contract
	net, err := renewalNet(in, c.Price)
	if err != nil {
		return renewal.Candidate{}, nil, err
	}
	best := renewal.Candidate{Price: c.Price, Probability: in.Response.Predict(in.Observation), Contribution: float64(net) - in.Cost, Eligible: true}
	if !in.Validated {
		return best, []renewal.Candidate{best}, nil
	}
	limit := min(c.Ceiling, c.Price+c.Price*c.MaxChangeBPS/10000)
	prices := []int64{c.Price, in.Public, limit}
	for step := int64(1); step <= 32; step++ {
		prices = append(prices, c.Price+(limit-c.Price)*step/32)
	}
	seen := map[int64]bool{}
	inputs := []compute.CandidateInput{}
	var addonBasis int64
	for _, a := range c.Addons {
		addonBasis += a.Price
	}
	for _, price := range prices {
		if seen[price] || price < addonBasis || price > limit || price < c.Price && price != in.Public {
			continue
		}
		seen[price] = true
		net, e := renewalNet(in, price)
		if e != nil {
			return best, nil, e
		}
		x := in.Observation
		x.LogPrice = math.Log(float64(max(1, price)) / float64(max(1, c.Price)))
		if in.Coupon != nil {
			x.CouponPresent = 1
			x.CouponDiscount = float64(price-net) / float64(max(1, price))
		}
		inputs = append(inputs, compute.CandidateInput{Price: price, NetPrice: net, Units: 1, Cost: in.Cost, Features: x})
	}
	out, err := s.PricingCalculate(ctx, compute.Task{Kind: "candidate_eval_v1", Model: in.Response, Candidates: inputs})
	if errors.Is(err, ErrPricingComputePending) {
		return best, nil, ErrPricingDecisionPending
	}
	if err != nil {
		return best, nil, err
	}
	utility := math.Inf(-1)
	curve := []renewal.Candidate{}
	for _, v := range out.Candidates {
		u := v.Probability*(v.Contribution+max(0, float64(net)-in.Cost)*in.Relationships*.9) - math.Abs(float64(v.Price-c.Price))*.05
		candidate := renewal.Candidate{Price: v.Price, Probability: v.Probability, Contribution: v.Contribution, Utility: u, Eligible: v.Contribution >= 0}
		curve = append(curve, candidate)
		if candidate.Eligible && u > utility {
			utility = u
			best = candidate
		}
	}
	return best, curve, nil
}
