package httpapi

import (
	"net/http"
)

func (s *Server) pricingRenewalInfo(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := chiURLParam(r, "id")
	c, err := s.deps.Store.PricingContract(r.Context(), user.ID, id)
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	purchase, err := s.deps.Store.PurchaseByID(r.Context(), id)
	if err != nil || purchase.UserID != user.ID {
		s.pricingError(w, r, err)
		return
	}
	p, err := s.deps.Store.PricingPolicy(r.Context(), "combo", purchase.ComboID)
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	q, err := s.deps.Store.PricingRenewalQuote(r.Context(), user.ID, id)
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contract": c, "quote": q, "policy": p, "graceSeconds": 259200})
}
