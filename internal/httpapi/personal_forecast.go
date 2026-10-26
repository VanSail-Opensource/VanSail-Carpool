package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) personalForecast(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	s.writePersonalForecast(w, r, user.ID)
}

func (s *Server) adminPersonalForecast(w http.ResponseWriter, r *http.Request) {
	s.writePersonalForecast(w, r, chi.URLParam(r, "id"))
}

func (s *Server) writePersonalForecast(w http.ResponseWriter, r *http.Request, user string) {
	privateSubscriptionHeaders(w)
	forecast, err := s.deps.Store.PersonalForecast(r.Context(), user, time.Now().UTC())
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, forecast)
}

func (s *Server) adminUserPrices(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	page, err := s.deps.Store.PricingUserPricePage(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("cursor"), time.Now().UTC())
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapUserPricePage(page))
}
