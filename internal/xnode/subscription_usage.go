package xnode

import (
	"context"
	"math/big"
	"strconv"
)

// subscriptionUsage projects existing accounting into client subscription info.
// Combo quota stays independent of PAYG inventory and wallet admission decisions.
func (s *Service) subscriptionUsage(ctx context.Context, identity Identity, result *SubscriptionMetadata) error {
	purchase, err := s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return err
	}
	if purchase != nil {
		until := purchase.ValidUntil
		result.ExpiresAt = &until
		result.LimitBytes = strconv.FormatInt(purchase.TrafficLimitBytes, 10)
		if identity.CurrentPeriodID == "" {
			return nil
		}
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return err
		}
		if period.PurchaseID == purchase.ID {
			result.UsedBytes = strconv.FormatInt(period.UsedBytes, 10)
			result.LimitBytes = strconv.FormatInt(period.LimitBytes, 10)
		}
		return nil
	}
	r, ok := s.paygRepository()
	if !ok {
		return nil
	}
	accounts, err := r.XNodePAYGAccounts(ctx, identity.UserID, "")
	if err != nil {
		return err
	}
	used, remaining := new(big.Int), new(big.Int)
	now := s.now()
	for _, account := range accounts {
		if account.UsedBytes < 0 || account.CreditBytes < 0 {
			return ErrInvalid
		}
		used.Add(used, big.NewInt(account.UsedBytes))
		if account.HasCredit(now) {
			remaining.Add(remaining, big.NewInt(account.CreditBytes))
		}
	}
	result.UsedBytes = used.String()
	result.LimitBytes = new(big.Int).Add(used, remaining).String()
	return nil
}
