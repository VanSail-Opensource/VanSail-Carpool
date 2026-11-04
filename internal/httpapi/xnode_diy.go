package httpapi

import (
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Server) xnodeDIY(w http.ResponseWriter, r *http.Request) {
	view, err := s.deps.XNode.OwnedDIY(r.Context(), currentUser(r).ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) xnodePreviewDIY(w http.ResponseWriter, r *http.Request) {
	var input xnode.DIYWrite
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	preview, err := s.deps.XNode.PreviewOwnedDIY(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) xnodeSaveDIY(w http.ResponseWriter, r *http.Request) {
	var input xnode.DIYWrite
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	result, err := s.deps.XNode.SaveOwnedDIY(r.Context(), currentUser(r).ID, key, input)
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}
