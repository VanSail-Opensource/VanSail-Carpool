package xnode

import "context"

// Hourly access has no purchased finite traffic quota: free/extra credits select
// speed only. Client subscription metadata must not present that pool as a hard
// traffic cap or derive expiry from the wallet balance.
func (s *Service) hourlySubscriptionUsage(ctx context.Context, owner string, result *SubscriptionMetadata) (bool, error) {
	principals, err := s.hourlyPrincipals(ctx, owner, "")
	if err != nil {
		return false, err
	}
	found := false
	for _, p := range principals {
		if !hourlyPrincipalAvailable(p, s.now()) {
			continue
		}
		found = true
		if result.ExpiresAt == nil || p.PaidThrough.After(*result.ExpiresAt) {
			until := p.PaidThrough
			result.ExpiresAt = &until
		}
	}
	if found {
		result.UsedBytes, result.LimitBytes = "0", "0"
	}
	return found, nil
}
