package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/onboarding"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) onboardingContent(w http.ResponseWriter, r *http.Request) {
	content, err := s.deps.Store.PublishedOnboarding(r.Context(), r.URL.Query().Get("locale"))
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "ONBOARDING_CONTENT_UNAVAILABLE", "Onboarding content could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, content)
}

func (s *Server) adminOnboardingBundle(w http.ResponseWriter, r *http.Request) {
	kind := chiURLParam(r, "kind")
	if kind != onboarding.KindWelcome && kind != onboarding.KindAgreements {
		s.writeError(w, r, http.StatusNotFound, "NOT_FOUND", "The onboarding bundle was not found.")
		return
	}
	bundle, err := s.deps.Store.OnboardingBundle(r.Context(), kind)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (s *Server) adminSaveOnboardingDraft(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(chiURLParam(r, "kind"))
	var request struct {
		DraftRevision int                `json:"draftRevision"`
		Content       onboarding.Content `json:"content"`
	}
	if err := decodeJSON(w, r, &request); err != nil || request.DraftRevision <= 0 {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ONBOARDING_CONTENT", "A current localized draft revision is required.")
		return
	}
	bundle, err := s.deps.Store.SaveOnboardingDraft(r.Context(), kind, request.Content, request.DraftRevision, currentUser(r).ID, time.Now().UTC())
	if err != nil {
		s.onboardingAdminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (s *Server) adminPublishOnboarding(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DraftRevision int `json:"draftRevision"`
	}
	if err := decodeJSON(w, r, &request); err != nil || request.DraftRevision <= 0 {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ONBOARDING_CONTENT", "A current draft revision is required.")
		return
	}
	bundle, err := s.deps.Store.PublishOnboarding(r.Context(), chiURLParam(r, "kind"), request.DraftRevision, currentUser(r).ID, time.Now().UTC())
	if err != nil {
		s.onboardingAdminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (s *Server) onboardingAdminFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, database.ErrConflict) {
		s.writeError(w, r, http.StatusConflict, "STALE_ONBOARDING_REVISION", "The onboarding draft changed. Reload before saving or publishing.")
		return
	}
	if errors.Is(err, onboarding.ErrInvalidContent) {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_ONBOARDING_CONTENT", "English and Simplified Chinese must have matching IDs and order.")
		return
	}
	s.adminFailure(w, r, err)
}
