package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) memberIPBlocks(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	items, err := s.deps.ConnectionDrops.List(r.Context(), user.ID)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "IP_BLOCKS_UNAVAILABLE", "Active IP blocks could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) memberUnblockIP(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.ConnectionDrops.Unblock(r.Context(), user.ID, user.ID, chiURLParam(r, "blockId"), key)
	if err != nil {
		s.writeIPBlockError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

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
