package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
)

func (s *Server) affiliateOverview(w http.ResponseWriter, r *http.Request) {
	result, err := s.deps.Affiliates.Overview(r.Context(), currentUser(r).ID)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "AFFILIATE_LOAD_FAILED", "Affiliate Centre could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) affiliateReferrals(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	result, err := s.deps.Affiliates.Referrals(r.Context(), currentUser(r).ID, page)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "AFFILIATE_REFERRALS_FAILED", "Referrals could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) adminAffiliates(w http.ResponseWriter, r *http.Request) {
	result, err := s.deps.Affiliates.Admin(r.Context())
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "AFFILIATE_ADMIN_LOAD_FAILED", "Affiliate settings could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) adminUpdateAffiliates(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input affiliates.ConfigInput
	if err := decoder.Decode(&input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_AFFILIATE_CONFIG", "Affiliate settings are invalid.")
		return
	}
	result, err := s.deps.Affiliates.Save(r.Context(), currentUser(r).ID, input)
	if errors.Is(err, affiliates.ErrVersionConflict) {
		s.writeError(w, r, http.StatusConflict, "AFFILIATE_VERSION_CONFLICT", "Affiliate settings changed. Reload and try again.")
		return
	}
	if errors.Is(err, affiliates.ErrInvalidInput) {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_AFFILIATE_CONFIG", "Affiliate settings are invalid.")
		return
	}
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "AFFILIATE_SAVE_FAILED", "Affiliate settings could not be saved.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
