package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
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
	if len(patch.PAYGSquads) > 1000 {
		s.writeError(w, r, 400, "INVALID_PREFERENCES", "Too many PAYG squad choices.")
		return
	}
	value, err := s.deps.Store.UpdateUserPreferences(r.Context(), user.ID, patch, time.Now().UTC())
	if errors.Is(err, database.ErrNotFound) {
		s.writeError(w, r, 400, "INVALID_PREFERENCES", "Choose an available PAYG squad.")
		return
	}
	if err != nil {
		s.writeError(w, r, 500, "PREFERENCES_UNAVAILABLE", "Preferences could not be saved.")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
