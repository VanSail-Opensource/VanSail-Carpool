package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountPricing(router chi.Router) {
	router.Get("/pricing", s.pricingDashboard)
	router.Get("/pricing/data", s.pricingData)
	router.Get("/pricing/history", s.pricingHistory)
	router.Get("/pricing/interventions/version", s.pricingScopeVersion)
	router.Put("/pricing/costs/{id}", s.pricingSaveCost)
	router.Post("/pricing/interventions", s.pricingIntervention)
	router.Post("/pricing/interventions/{id}/cancel", s.pricingCancelIntervention)
	router.Post("/pricing/exports/authorization", s.pricingExportAuthorization)
	router.Post("/pricing/exports", s.pricingExport)
	router.Get("/pricing/products/{kind}/{id}", s.pricingPolicy)
	router.Put("/pricing/products/{kind}/{id}", s.savePricingPolicy)
}

func (s *Server) pricingPolicy(w http.ResponseWriter, r *http.Request) {
	p, err := s.deps.Store.PricingPolicy(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"))
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) savePricingPolicy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Policy pricing.Policy `json:"policy"`
		Reason string         `json:"reason"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.pricingError(w, r, err)
		return
	}
	input.Policy.ProductType = chi.URLParam(r, "kind")
	input.Policy.ProductID = chi.URLParam(r, "id")
	p, err := s.deps.Store.SavePricingPolicy(r.Context(), currentUser(r).ID, input.Reason, input.Policy, time.Now().UTC())
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) pricingError(w http.ResponseWriter, r *http.Request, err error) {
	var code pricing.Code
	if errors.As(err, &code) {
		s.writeError(w, r, http.StatusConflict, string(code), string(code))
		return
	}
	if errors.Is(err, database.ErrNotFound) {
		s.writeError(w, r, http.StatusNotFound, "PRICING_NOT_FOUND", "Pricing resource was not found.")
		return
	}
	s.deps.Logger.Warn("pricing request failed", "error", err)
	s.writeError(w, r, http.StatusUnprocessableEntity, "PRICING_INVALID_REQUEST", "Pricing request is invalid or no longer eligible.")
}
