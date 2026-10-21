package httpapi

import (
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Server) xnodeStartRouteSpeedtest(w http.ResponseWriter, r *http.Request) {
	var input xnode.RouteSpeedtestRequest
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.XNode.QueueRouteSpeedtest(r.Context(), currentUser(r).ID, key, input)
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) xnodeRouteSpeedtestResult(w http.ResponseWriter, r *http.Request) {
	result, err := s.deps.XNode.OwnedRouteSpeedtest(r.Context(), currentUser(r).ID, chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}
