package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) adminCoupons(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Coupons.List(r.Context(), false)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	response := make([]couponResponse, 0, len(items))
	for _, item := range items {
		response = append(response, mapCoupon(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response})
}

type couponRequest struct {
	Code             *string                         `json:"code"`
	Name             *string                         `json:"name"`
	Kind             *coupons.CouponKind             `json:"kind"`
	DiscountMode     *coupons.DiscountMode           `json:"discountMode"`
	ValueMinorOrBPS  *string                         `json:"valueMinorOrBps"`
	PercentCapMinor  nullableRequestField[string]    `json:"percentCapMinor"`
	EligibleComboIDs *[]string                       `json:"eligibleComboIds"`
	EligibleSquadIDs *[]string                       `json:"eligibleSquadIds"`
	ExpiresAt        nullableRequestField[time.Time] `json:"expiresAt"`
	GlobalUseLimit   nullableRequestField[int64]     `json:"globalUseLimit"`
	PerUserUseLimit  nullableRequestField[int64]     `json:"perUserUseLimit"`
	Active           *bool                           `json:"active"`
}

func (s *Server) adminCreateCoupon(w http.ResponseWriter, r *http.Request) {
	s.adminSaveCoupon(w, r, "")
}

func (s *Server) adminUpdateCoupon(w http.ResponseWriter, r *http.Request) {
	s.adminSaveCoupon(w, r, chiURLParam(r, "id"))
}

func (s *Server) adminDeactivateCoupon(w http.ResponseWriter, r *http.Request) {
	active := false
	input, err := s.couponInput(r.Context(), chiURLParam(r, "id"), couponRequest{Active: &active})
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	if _, err := s.deps.Coupons.Save(r.Context(), input); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminSaveCoupon(w http.ResponseWriter, r *http.Request, id string) {
	var request couponRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COUPON", "Coupon fields are invalid.")
		return
	}
	input, err := s.couponInput(r.Context(), id, request)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	item, err := s.deps.Coupons.Save(r.Context(), input)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, mapCoupon(item))
}

func (s *Server) couponInput(ctx context.Context, id string, request couponRequest) (coupons.CouponInput, error) {
	var input coupons.CouponInput
	if id != "" {
		items, err := s.deps.Coupons.List(ctx, false)
		if err != nil {
			return coupons.CouponInput{}, err
		}
		found := false
		for _, item := range items {
			if item.ID == id {
				input = item.CouponInput
				found = true
				break
			}
		}
		if !found {
			return coupons.CouponInput{}, database.ErrNotFound
		}
	}
	input.ID = id
	if request.Code != nil {
		input.Code = *request.Code
	}
	if request.Name != nil {
		input.Name = *request.Name
	}
	if request.Kind != nil {
		input.Kind = *request.Kind
	}
	if request.DiscountMode != nil {
		input.DiscountMode = *request.DiscountMode
	}
	if request.ValueMinorOrBPS != nil {
		value, err := parseMinorString(*request.ValueMinorOrBPS, true)
		if err != nil {
			return coupons.CouponInput{}, coupons.ErrInvalidInput
		}
		input.ValueMinorOrBPS = value
	}
	if request.PercentCapMinor.Set {
		input.PercentCapMinor = nil
		if request.PercentCapMinor.Value != nil {
			value, err := parseMinorString(*request.PercentCapMinor.Value, true)
			if err != nil {
				return coupons.CouponInput{}, coupons.ErrInvalidInput
			}
			input.PercentCapMinor = &value
		}
	}
	if request.EligibleComboIDs != nil {
		input.EligibleComboIDs = *request.EligibleComboIDs
	}
	if request.EligibleSquadIDs != nil {
		input.EligibleSquadIDs = *request.EligibleSquadIDs
	}
	if request.ExpiresAt.Set {
		input.ExpiresAt = request.ExpiresAt.Value
	}
	if request.GlobalUseLimit.Set {
		input.GlobalUseLimit = request.GlobalUseLimit.Value
	}
	if request.PerUserUseLimit.Set {
		input.PerUserUseLimit = request.PerUserUseLimit.Value
	}
	if request.Active != nil {
		input.Active = *request.Active
	}
	return input, nil
}
