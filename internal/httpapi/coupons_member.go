package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

type couponResponse struct {
	ID               string               `json:"id"`
	Code             string               `json:"code"`
	Name             string               `json:"name"`
	Kind             coupons.CouponKind   `json:"kind"`
	DiscountMode     coupons.DiscountMode `json:"discountMode,omitempty"`
	ValueMinorOrBPS  string               `json:"valueMinorOrBps"`
	PercentCapMinor  *string              `json:"percentCapMinor,omitempty"`
	EligibleComboIDs []string             `json:"eligibleComboIds"`
	EligibleSquadIDs []string             `json:"eligibleSquadIds"`
	ExpiresAt        *time.Time           `json:"expiresAt"`
	GlobalUseLimit   *int64               `json:"globalUseLimit"`
	PerUserUseLimit  *int64               `json:"perUserUseLimit"`
	Active           bool                 `json:"active"`
	UsageCount       int64                `json:"usageCount"`
	CreatedAt        time.Time            `json:"createdAt"`
	UpdatedAt        time.Time            `json:"updatedAt"`
}

type couponGrantResponse struct {
	ID         string         `json:"id"`
	Coupon     couponResponse `json:"coupon"`
	SourceType string         `json:"sourceType"`
	SourceID   string         `json:"sourceId"`
	Status     string         `json:"status"`
	UseCount   int64          `json:"useCount"`
	CreatedAt  time.Time      `json:"createdAt"`
	ConsumedAt *time.Time     `json:"consumedAt"`
}

type couponRedemptionResponse struct {
	ID                string               `json:"id"`
	Coupon            couponResponse       `json:"coupon"`
	Grant             *couponGrantResponse `json:"grant"`
	BalanceDeltaMinor string               `json:"balanceDeltaMinor"`
	BalanceAfterMinor string               `json:"balanceAfterMinor"`
	IdempotencyKey    string               `json:"idempotencyKey"`
	Replayed          bool                 `json:"replayed"`
	CreatedAt         time.Time            `json:"createdAt"`
}

func mapCoupon(coupon coupons.Coupon) couponResponse {
	var capMinor *string
	if coupon.PercentCapMinor != nil {
		value := strconv.FormatInt(*coupon.PercentCapMinor, 10)
		capMinor = &value
	}
	return couponResponse{ID: coupon.ID, Code: coupon.Code, Name: coupon.Name, Kind: coupon.Kind, DiscountMode: coupon.DiscountMode,
		ValueMinorOrBPS: strconv.FormatInt(coupon.ValueMinorOrBPS, 10), PercentCapMinor: capMinor,
		EligibleComboIDs: nonNilStrings(coupon.EligibleComboIDs), EligibleSquadIDs: nonNilStrings(coupon.EligibleSquadIDs),
		ExpiresAt: coupon.ExpiresAt, GlobalUseLimit: coupon.GlobalUseLimit, PerUserUseLimit: coupon.PerUserUseLimit,
		Active: coupon.Active, UsageCount: coupon.UsageCount, CreatedAt: coupon.CreatedAt, UpdatedAt: coupon.UpdatedAt}
}

func mapCouponGrant(grant coupons.Grant) couponGrantResponse {
	return couponGrantResponse{ID: grant.ID, Coupon: mapCoupon(grant.Coupon), SourceType: grant.SourceType, SourceID: grant.SourceID,
		Status: grant.Status, UseCount: grant.UseCount, CreatedAt: grant.CreatedAt, ConsumedAt: grant.ConsumedAt}
}

func (s *Server) couponWallet(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	grants, err := s.deps.Coupons.Wallet(r.Context(), user.ID)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	items := make([]couponGrantResponse, 0, len(grants))
	for _, grant := range grants {
		items = append(items, mapCouponGrant(grant))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) couponRedeem(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COUPON_REQUEST", "A coupon code is required.")
		return
	}
	result, err := s.deps.Coupons.Redeem(r.Context(), user.ID, request.Code, key)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	response := couponRedemptionResponse{ID: result.ID, Coupon: mapCoupon(result.Coupon), BalanceDeltaMinor: strconv.FormatInt(result.BalanceDeltaMinor, 10),
		BalanceAfterMinor: strconv.FormatInt(result.BalanceAfterMinor, 10), IdempotencyKey: result.IdempotencyKey,
		Replayed: result.Replayed, CreatedAt: result.CreatedAt}
	if result.Grant != nil {
		grant := mapCouponGrant(*result.Grant)
		response.Grant = &grant
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) couponDiscard(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	if err := s.deps.Coupons.Discard(r.Context(), user.ID, chiURLParam(r, "id")); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
