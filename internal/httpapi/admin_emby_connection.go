package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) adminEmbyConnection(w http.ResponseWriter, r *http.Request) {
	view, err := s.deps.Admin.EmbyConnection(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) adminSaveEmbyConnection(w http.ResponseWriter, r *http.Request) {
	var input admin.EmbyConnectionWrite
	if err := decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_EMBY_CONNECTION", "An Emby connection is required.")
		return
	}
	view, err := s.deps.Admin.SaveEmbyConnection(r.Context(), currentUser(r).ID, input)
	switch {
	case errors.Is(err, database.ErrConflict):
		s.writeError(w, r, http.StatusConflict, "EMBY_CONNECTION_CHANGED", "Reload the Emby connection before saving.")
	case errors.Is(err, admin.ErrEmbyFreshToken):
		s.writeError(w, r, http.StatusUnprocessableEntity, "EMBY_FRESH_TOKEN_REQUIRED", "Changing the Emby address requires a fresh API token.")
	case errors.Is(err, admin.ErrEmbyConnection):
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_EMBY_CONNECTION", "A valid HTTPS address and API token are required.")
	case err != nil:
		s.adminFailure(w, r, err)
	default:
		writeJSON(w, http.StatusOK, view)
	}
}
