package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) cancelQueuedPurchase(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	purchase, err := s.deps.Catalog.CancelQueuedPurchase(r.Context(), user, chiURLParam(r, "id"))
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			s.writeError(w, r, http.StatusNotFound, "PURCHASE_NOT_FOUND", "The queued ride could not be found.")
		case errors.Is(err, database.ErrConflict):
			s.writeError(w, r, http.StatusConflict, "PURCHASE_CANCEL_CONFLICT", "The queued ride is no longer available for cancellation.")
		default:
			s.writeError(w, r, http.StatusInternalServerError, "PURCHASE_CANCEL_FAILED", "The queued ride could not be cancelled.")
		}
		return
	}
	writeJSON(w, http.StatusOK, purchase)
}
