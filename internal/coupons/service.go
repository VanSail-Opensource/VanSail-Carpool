package coupons

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Store is the narrow persistence contract required by Service.
type Store interface {
	SaveCoupon(context.Context, CouponInput, time.Time) (Coupon, error)
	SaveAdminCoupon(context.Context, string, CouponInput, time.Time, time.Time) (Coupon, error)
	ListCoupons(context.Context, bool) ([]Coupon, error)
	RedeemCoupon(context.Context, string, string, string, time.Time) (RedemptionResult, error)
	GrantCoupon(context.Context, string, string, string, string, time.Time) (Grant, error)
	ListCouponGrants(context.Context, string, time.Time) ([]Grant, error)
	DiscardCouponGrant(context.Context, string, string, time.Time) error
	QuotePurchaseCoupon(context.Context, PurchaseContext, time.Time) (Discount, error)
}

// Service validates coupon requests and delegates atomic effects to storage.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService constructs the coupon application service.
func NewService(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

// Save is the raw definition writer retained for internal fixtures.
// Administrator handlers use SaveAdmin for atomic authorization, revision and audit.
func (service *Service) Save(ctx context.Context, input CouponInput) (Coupon, error) {
	normalized, err := input.Normalize()
	if err != nil {
		return Coupon{}, err
	}
	return service.store.SaveCoupon(ctx, normalized, service.now().UTC())
}

// SaveAdmin validates a definition and binds a partial edit to the revision read
// before merging it. Creation uses a zero expectedUpdatedAt.
func (service *Service) SaveAdmin(ctx context.Context, actorID string, input CouponInput, expectedUpdatedAt time.Time) (Coupon, error) {
	normalized, err := input.Normalize()
	if err != nil {
		return Coupon{}, err
	}
	return service.store.SaveAdminCoupon(ctx, actorID, normalized, expectedUpdatedAt, service.now().UTC())
}

// List returns active member-visible coupons or all administrator definitions.
func (service *Service) List(ctx context.Context, activeOnly bool) ([]Coupon, error) {
	return service.store.ListCoupons(ctx, activeOnly)
}

// Redeem canonicalizes a code and atomically grants or applies it.
func (service *Service) Redeem(ctx context.Context, userID, code, idempotencyKey string) (RedemptionResult, error) {
	canonical, err := CanonicalCode(code)
	if err != nil {
		return RedemptionResult{}, err
	}
	if strings.TrimSpace(userID) == "" || !validIdempotencyKey(idempotencyKey) {
		return RedemptionResult{}, fmt.Errorf("%w: incomplete redemption request", ErrInvalidInput)
	}
	return service.store.RedeemCoupon(ctx, userID, canonical, idempotencyKey, service.now().UTC())
}

// Wallet returns currently usable purchase-discount grants.
func (service *Service) Wallet(ctx context.Context, userID string) ([]Grant, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: missing user", ErrInvalidInput)
	}
	return service.store.ListCouponGrants(ctx, userID, service.now().UTC())
}

// Discard hides one active member grant without deleting its redemption or
// purchase history. Repeating the same discard is safe.
func (service *Service) Discard(ctx context.Context, userID, grantID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(grantID) == "" {
		return fmt.Errorf("%w: missing coupon grant", ErrInvalidInput)
	}
	return service.store.DiscardCouponGrant(ctx, userID, grantID, service.now().UTC())
}

// Quote calculates one explicitly selected grant against a server-priced basket.
func (service *Service) Quote(ctx context.Context, input PurchaseContext) (Discount, error) {
	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.GrantID) == "" || strings.TrimSpace(input.ComboID) == "" || input.GrossPriceMinor < 0 {
		return Discount{}, fmt.Errorf("%w: incomplete coupon quote", ErrInvalidInput)
	}
	input.AddonSquadIDs = uniqueSorted(input.AddonSquadIDs)
	return service.store.QuotePurchaseCoupon(ctx, input, service.now().UTC())
}

func validIdempotencyKey(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 128
}
