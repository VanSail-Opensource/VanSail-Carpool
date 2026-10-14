package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// GrantCoupon adds one active purchase-discount coupon to an administrator-selected wallet.
func (s *UserWorkflows) GrantCoupon(ctx context.Context, actorID, userID, couponID, key, reason string) (coupons.Grant, error) {
	reason, couponID = strings.TrimSpace(reason), strings.TrimSpace(couponID)
	if !validCommand(actorID, key, reason) || strings.TrimSpace(userID) == "" || couponID == "" {
		return coupons.Grant{}, errors.New("invalid admin coupon grant")
	}
	return s.repository.GrantAdminCoupon(ctx, database.AdminCouponGrantInput{ActorUserID: actorID, UserID: userID,
		CouponID: couponID, IdempotencyKey: strings.TrimSpace(key), Reason: reason}, s.now().UTC())
}

// DiscardCoupon retains the grant history while removing it from the active wallet.
func (s *UserWorkflows) DiscardCoupon(ctx context.Context, actorID, userID, grantID, key string) error {
	if strings.TrimSpace(actorID) == "" || strings.TrimSpace(userID) == "" || strings.TrimSpace(grantID) == "" || strings.TrimSpace(key) == "" {
		return errors.New("invalid admin coupon discard")
	}
	return s.repository.DiscardAdminCoupon(ctx, actorID, userID, grantID, key, s.now().UTC())
}
