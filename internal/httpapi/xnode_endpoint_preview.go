package httpapi

import (
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Server) xnodePreviewEndpoint(w http.ResponseWriter, r *http.Request) {
	var input xnode.Endpoint
	if err := decodeJSON(w, r, &input); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	result, err := s.deps.XNode.PreviewEndpoint(r.Context(), input)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}
