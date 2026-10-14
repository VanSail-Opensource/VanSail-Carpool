package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
	"net/http"
	"time"
)

func (s *Server) userPreferences(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	value, err := s.deps.Store.UserPreferenceSnapshot(r.Context(), user.ID, time.Now().UTC())
	if err != nil {
		s.writeError(w, r, 500, "PREFERENCES_UNAVAILABLE", "Preferences could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateUserPreferences(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var patch preferences.Patch
	if err := decodeJSON(w, r, &patch); err != nil {
		s.writeError(w, r, 400, "INVALID_PREFERENCES", "Provide valid preference fields.")
		return
	}
	value, err := s.deps.Store.UpdateUserPreferences(r.Context(), user.ID, patch, time.Now().UTC())
	if err != nil {
		s.writeError(w, r, 500, "PREFERENCES_UNAVAILABLE", "Preferences could not be saved.")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
