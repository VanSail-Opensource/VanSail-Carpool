package database

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func TestRenewalContributionAndRelationshipUseProbabilityOnce(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	in := renewalPriceSnapshot{Contract: renewal.Contract{Price: 1000, Ceiling: 1100, MaxChangeBPS: 1000}, Public: 1000, Validated: true, Cost: 100, Relationships: 2, Response: pricing.ResponseModel{Validated: true}, Support: &pricing.PriceSupport{Samples: 200, Low: math.Log(.8), High: math.Log(1.2)}}
	if _, _, err := s.calculateRenewalPrice(ctx, in); !errors.Is(err, ErrPricingDecisionPending) {
		t.Fatal(err)
	}
	job, err := s.ClaimComputeOutboxJob(ctx, now.Add(time.Second))
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	var payload struct {
		InputID string `json:"inputId"`
	}
	if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	task, err := s.PricingComputeInput(ctx, payload.InputID)
	if err != nil {
		t.Fatal(err)
	}
	results, err := compute.Run(ctx, compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AcceptPricingCompute(ctx, task, results.Items[0], now); err != nil {
		t.Fatal(err)
	}
	_, curve, err := s.calculateRenewalPrice(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range curve {
		expected := .5 * (float64(candidate.Price) - 100)
		if math.Abs(candidate.Contribution-expected) > 1e-9 || math.Abs(candidate.Utility-(expected+.5*900*2*.9)) > 1e-9 {
			t.Fatal(candidate)
		}
	}
}
