package database

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
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
	base := in.Observation
	base.LogPrice = 0
	if in.Coupon != nil {
		base.CouponPresent = 1
		base.CouponDiscount = float64(c.Price-net) / float64(max(1, c.Price))
	}
	probability := in.Response.Predict(base)
	best := renewal.Candidate{Price: c.Price, Probability: probability, Contribution: probability * (float64(net) - in.Cost), Eligible: true}
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
		if price != c.Price && (in.Support == nil || !in.Support.Allows(price, c.Price, in.Coupon != nil)) {
			continue
		}
		inputs = append(inputs, compute.CandidateInput{Price: price, NetPrice: net, Units: 1, Cost: in.Response.Predict(x) * in.Cost, Features: x})
	}
	out, err := s.PricingCalculate(ctx, compute.Task{Kind: "candidate_eval_v1", Model: in.Response, Candidates: inputs})
	if errors.Is(err, ErrPricingComputePending) {
		return best, nil, ErrPricingDecisionPending
	}
	if err != nil {
		return best, nil, err
	}
	curve := []renewal.Candidate{}
	scores := []pricing.PriceCandidate{}
	for _, v := range out.Candidates {
		u := v.Contribution + v.Probability*max(0, float64(net)-in.Cost)*in.Relationships*.9
		candidate := renewal.Candidate{Price: v.Price, Probability: v.Probability, Contribution: v.Contribution, Utility: u, Eligible: v.Contribution >= 0}
		curve = append(curve, candidate)
		scores = append(scores, pricing.PriceCandidate{Price: v.Price, Probability: v.Probability, Contribution: u, Eligible: candidate.Eligible})
	}
	if selected, ok := pricing.StableCandidate(c.Price, scores); ok {
		for _, candidate := range curve {
			if candidate.Price == selected.Price {
				best = candidate
				break
			}
		}
	}
	return best, curve, nil
}
