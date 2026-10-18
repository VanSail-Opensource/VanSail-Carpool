package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) xnodeGrantFirewallExemption(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input xnode.AdminFirewallExemption
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	input.Key = key
	receipt, err := s.deps.XNode.GrantFirewallExemption(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusAccepted, receipt)
}
