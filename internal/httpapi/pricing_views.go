package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) pricingDashboard(w http.ResponseWriter, r *http.Request) {
	items, next, err := s.deps.Store.PricingProducts(r.Context(), r.URL.Query().Get("cursor"))
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	storage, err := s.deps.Store.PricingStorage(r.Context())
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	summary, err := s.deps.Store.PricingSummary(r.Context(), time.Now().UTC())
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary, "products": items, "nextCursor": next, "storage": storage, "health": s.deps.Store.PricingHealth().Snapshot(time.Now().UTC()), "datasets": pricingDatasetNames()})
}

func pricingDatasetNames() []string {
	return []string{"pricing/personal_decisions", "features/features", "forecasts/historical_predictions", "forecasts/prediction_evaluations", "forecasts/renewal_predictions", "forecasts/model_versions", "pricing/published_prices", "pricing/pricing_decisions", "pricing/contracts", "pricing/provider_costs", "pricing/provider_cost_versions", "validation/calibration", "pricing/discount_decisions", "raw/offers", "operations/health_incidents", "operations/policy_events", "operations/admin_overrides", "validation/model_metrics"}
}

func (s *Server) pricingData(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("dataset")
	if name == "pricing/eligible_offers" {
		page, err := s.deps.Store.PricingUserPricePage(r.Context(), r.URL.Query().Get("user"), r.URL.Query().Get("cursor"), time.Now().UTC())
		if err != nil {
			s.pricingError(w, r, err)
			return
		}
		items := []map[string]any{}
		for _, p := range page.Items {
			trace, _ := json.Marshal(p)
			items = append(items, map[string]any{"product_type": p.ProductType, "product_id": p.ProductID, "public_minor": p.PublicPrice.Minor, "eligible_minor": p.PersonalPrice.Minor, "net_minor": p.NetPrice.Minor, "state": p.Decision.Status, "reason": p.Decision.Reason, "price_version": p.Decision.Version, "trace_json": string(trace)})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": 100, "truncated": page.NextCursor != "", "nextCursor": page.NextCursor})
		return
	}
	allowed := false
	for _, known := range pricingDatasetNames() {
		if name == known {
			allowed = true
		}
	}
	if !allowed {
		s.writeError(w, r, http.StatusBadRequest, "PRICING_INVALID_REQUEST", "Unknown pricing dataset.")
		return
	}
	scope := database.PricingExportScope{IncludeUsers: true, IncludeFinancial: true}
	if value := r.URL.Query().Get("user"); value != "" {
		scope.Users = []string{value}
	}
	if value := r.URL.Query().Get("node"); value != "" {
		scope.Nodes = []string{value}
	}
	if value := r.URL.Query().Get("product"); value != "" {
		scope.Products = []string{value}
	}
	if value := r.URL.Query().Get("from"); value != "" {
		at, err := time.Parse(time.RFC3339, value)
		if err != nil {
			s.pricingError(w, r, err)
			return
		}
		scope.From = at
	}
	if value := r.URL.Query().Get("until"); value != "" {
		at, err := time.Parse(time.RFC3339, value)
		if err != nil {
			s.pricingError(w, r, err)
			return
		}
		scope.Until = at
	}
	items, err := s.deps.Store.ReadPricingDataset(r.Context(), name, scope, 500)
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": 500, "truncated": len(items) == 500})
}

func (s *Server) pricingIntervention(w http.ResponseWriter, r *http.Request) {
	var in database.PricingIntervention
	if err := decodeJSON(w, r, &in); err != nil {
		s.pricingError(w, r, err)
		return
	}
	item, err := s.deps.Store.CreatePricingIntervention(r.Context(), currentUser(r).ID, in, time.Now().UTC())
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) pricingCancelIntervention(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &input); err != nil || strings.TrimSpace(input.Reason) == "" {
		s.writeError(w, r, http.StatusBadRequest, "PRICING_INVALID_REQUEST", "A reason is required.")
		return
	}
	if err := s.deps.Store.CancelPricingIntervention(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), input.Reason, time.Now().UTC()); err != nil {
		s.pricingError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
