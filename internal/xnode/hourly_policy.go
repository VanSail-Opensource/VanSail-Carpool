package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// HourlyAccountPolicy is authoritative eligibility supplied by commerce. The
// owner ID retains the existing opaque users.id representation.
type HourlyAccountPolicy struct {
	OwnerID             string
	AccountGeneration   int64
	PaidThrough         time.Time
	RateBytesPerSecond  uint64
	DailyAllowanceBytes int64
	NodePaidThrough     map[string]time.Time
}

// HourlyPolicyRepository is optional until hourly enrollment is implemented.
// Missing support or an empty response leaves hourly sales/enforcement ineligible.
type HourlyPolicyRepository interface {
	HourlyEnforcementPolicies(context.Context, time.Time) ([]HourlyAccountPolicy, error)
}

func (m *Master) refreshHourlyPolicies(ctx context.Context) {
	co := m.hourly
	policies, err := m.service.enforcementHourlyPolicies(ctx, m.service.now())
	if err != nil {
		policies = nil
	}
	now := m.service.now()
	next := map[string]HourlyAccountPolicy{}
	duplicates := map[string]bool{}
	seen := map[string]bool{}
	if len(policies) <= 8192 {
		for _, policy := range policies {
			if seen[policy.OwnerID] {
				duplicates[policy.OwnerID] = true
			}
			seen[policy.OwnerID] = true
			if policy.OwnerID == "" || policy.AccountGeneration < 0 || !policy.PaidThrough.After(now) || policy.RateBytesPerSecond > 1_000_000_000_000 || policy.DailyAllowanceBytes < 0 || len(policy.NodePaidThrough) > 128 {
				continue
			}
			copy := policy
			copy.NodePaidThrough = map[string]time.Time{}
			for node, paid := range policy.NodePaidThrough {
				if paid.After(now) {
					copy.NodePaidThrough[node] = paid
				}
			}
			next[policy.OwnerID] = copy
		}
	}
	for id := range duplicates {
		delete(next, id)
	}
	co.policyMu.Lock()
	co.policies, co.policyAt = next, now
	co.policyMu.Unlock()
}

func (m *Master) hourlyPolicy(owner wire.HourlyOwner, nodeID string, at time.Time) (HourlyAccountPolicy, bool) {
	m.mu.RLock()
	co := m.hourly
	m.mu.RUnlock()
	if co == nil {
		return HourlyAccountPolicy{}, false
	}
	co.policyMu.RLock()
	defer co.policyMu.RUnlock()
	policy, ok := co.policies[owner.ID]
	if !ok || at.Before(co.policyAt) || at.Sub(co.policyAt) > 4*time.Second || uint64(policy.AccountGeneration) != owner.Generation || !policy.PaidThrough.After(at) || !policy.NodePaidThrough[nodeID].After(at) {
		return HourlyAccountPolicy{}, false
	}
	copy := policy
	copy.NodePaidThrough = map[string]time.Time{}
	for node, paid := range policy.NodePaidThrough {
		copy.NodePaidThrough[node] = paid
	}
	return copy, true
}

func (co *hourlyCoordinator) allocatorPolicies(now time.Time) []hourly.Policy {
	co.policyMu.RLock()
	defer co.policyMu.RUnlock()
	if now.Before(co.policyAt) || now.Sub(co.policyAt) > 4*time.Second {
		return nil
	}
	result := make([]hourly.Policy, 0, len(co.policies))
	for _, policy := range co.policies {
		result = append(result, hourly.Policy{Owner: hourly.Owner{ID: policy.OwnerID, Generation: uint64(policy.AccountGeneration)}, PaidThrough: policy.PaidThrough, RateBytesPerSecond: policy.RateBytesPerSecond, BurstBytes: hourly.MaxBurstBytes})
	}
	return result
}
