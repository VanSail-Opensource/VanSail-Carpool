package hourly

import "time"

// CanAdmit is a read-only capacity check, not a grant or reservation. Current
// paid owners retain future capacity even before their first demand. Retired
// allocator state remains counted until the normal Tick/input cleanup removes
// it; inspecting readiness never changes expiry or pruning timing. Empty owner
// asks whether a new owner could fit, without granting an anonymous identity.
func (a *Allocator) CanAdmit(now time.Time, owner Owner, policies []Policy) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if now.IsZero() || now.Before(a.lastNow) || now.Before(a.quietUntil) || owner.ID != "" && !validOwner(owner) {
		return false
	}
	if state := a.accounts[owner.ID]; state != nil {
		return owner.Generation >= state.generation
	}
	owners := make(map[string]bool, min(a.limits.MaxAccounts+1, len(a.accounts)+len(policies)))
	for id := range a.accounts {
		owners[id] = true
	}
	for _, policy := range policies {
		if !validOwner(policy.Owner) || !policy.PaidThrough.After(now) || policy.RateBytesPerSecond > 1_000_000_000_000 || policy.BurstBytes > MaxBurstBytes {
			continue
		}
		if policy.Owner.ID == owner.ID && policy.Owner.Generation > owner.Generation {
			return false
		}
		owners[policy.Owner.ID] = true
		if len(owners) > a.limits.MaxAccounts {
			return false
		}
	}
	if owner.ID != "" && owners[owner.ID] {
		return true
	}
	return len(owners) < a.limits.MaxAccounts
}
