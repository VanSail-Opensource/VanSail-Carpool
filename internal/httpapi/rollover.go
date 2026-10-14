package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) rolloverProjection(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	projection, err := s.deps.Catalog.RolloverProjection(r.Context(), user, chiURLParam(r, "id"))
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			s.writeError(w, r, http.StatusNotFound, "PURCHASE_NOT_FOUND", "The requested ride could not be found.")
		case errors.Is(err, catalog.ErrRolloverNotActive):
			s.writeError(w, r, http.StatusConflict, "ROLLOVER_NOT_ACTIVE", "Rollover details are available for the active ride only.")
		default:
			s.deps.Logger.Warn("load live rollover projection", "request_id", middlewareRequestID(r), "user_id", user.ID, "purchase_id", chiURLParam(r, "id"), "error", err)
			s.writeError(w, r, http.StatusBadGateway, "ROLLOVER_UNAVAILABLE", "Live rollover details are temporarily unavailable.")
		}
		return
	}
	writeJSON(w, http.StatusOK, projection)
}
