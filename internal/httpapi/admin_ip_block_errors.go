package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"net/http"
)

func (s *Server) writeIPBlockError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, connections.ErrIPBlockNotFound), errors.Is(err, database.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "IP_BLOCK_NOT_FOUND", "The active IP block could not be found.")
	case errors.Is(err, database.ErrConflict):
		s.writeError(w, r, http.StatusConflict, "IP_BLOCK_CONFLICT", "This IP block already has an open operation.")
	default:
		s.writeError(w, r, http.StatusInternalServerError, "IP_BLOCK_OPERATION_FAILED", "The IP block operation could not be created.")
	}
}
