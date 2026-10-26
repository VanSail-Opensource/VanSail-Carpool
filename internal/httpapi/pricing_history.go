package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) pricingHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	horizon, err := strconv.ParseInt(q.Get("horizon"), 10, 64)
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	f := database.PricingHistoryFilter{EntityType: q.Get("entityType"), EntityID: q.Get("entityId"), Metric: q.Get("metric"), ModelVersion: q.Get("modelVersion"), Horizon: horizon}
	if value := q.Get("from"); value != "" {
		f.From, err = time.Parse(time.RFC3339, value)
		if err != nil {
			s.pricingError(w, r, err)
			return
		}
	}
	if value := q.Get("until"); value != "" {
		f.Until, err = time.Parse(time.RFC3339, value)
		if err != nil {
			s.pricingError(w, r, err)
			return
		}
	}
	items, metrics, err := s.deps.Store.PricingHistory(r.Context(), f, time.Now().UTC())
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "metrics": metrics, "horizonBandSeconds": 3600})
}

func (s *Server) pricingScopeVersion(w http.ResponseWriter, r *http.Request) {
	version, err := s.deps.Store.PricingScopeVersion(r.Context(), r.URL.Query().Get("scopeType"), r.URL.Query().Get("scopeId"))
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": version})
}

func (s *Server) pricingSaveCost(w http.ResponseWriter, r *http.Request) {
	var in database.PricingCost
	if err := decodeJSON(w, r, &in); err != nil {
		s.pricingError(w, r, err)
		return
	}
	in.NodeID = chiURLParam(r, "id")
	if err := s.deps.Store.SavePricingCost(r.Context(), currentUser(r).ID, in, time.Now().UTC()); err != nil {
		s.pricingError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
