package hourlybilling

import (
	"context"
	"errors"
	"time"
)

func (s *Service) settleOwner(ctx context.Context, target settlementTarget) error {
	if s.native == nil {
		return wait("HOURLY_NATIVE_PENDING", ErrUnavailable)
	}
	snapshot, err := s.policy.HourlyPolicy(ctx)
	if err != nil {
		return wait("HOURLY_POLICY_PENDING", err)
	}
	enrollments, err := s.store.ListHourlyEnrollments(ctx, target.OwnerID, target.Generation, 200)
	if err != nil {
		return wait("HOURLY_OWNER_PENDING", err)
	}
	guards := s.deliveryGuards(ctx, enrollments)
	settleCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	result, err := s.store.SettleHourlyOwnerAvailable(settleCtx, target.OwnerID, target.Generation, snapshot, guards, s.now())
	cancel()
	if err != nil {
		return wait("HOURLY_SETTLEMENT_PENDING", err)
	}
	retryAt := s.now()
	pending := s.applySettlements(ctx, settlementApplications(result))
	if pending != nil {
		return wait("HOURLY_DELIVERY_PENDING", errors.Join(ErrUnavailable, pending))
	}
	if len(result.SkippedEnrollmentIDs) > 0 {
		return deliveryWait("HOURLY_DELIVERY_PENDING", ErrUnavailable, guards, result.SkippedEnrollmentIDs, retryAt)
	}
	return nil
}
