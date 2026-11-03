package httpapi

import (
	"net/http"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Server) xnodeAutomaticFirewallLeases(w http.ResponseWriter, r *http.Request) {
	page, size := 1, 50
	for key, target := range map[string]*int{"page": &page, "pageSize": &size} {
		if raw := r.URL.Query().Get(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				s.xnodeError(w, r, xnode.ErrInvalid)
				return
			}
			*target = value
		}
	}
	result, err := s.deps.XNode.AutomaticFirewallLeases(r.Context(), r.URL.Query().Get("query"), page, size)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}
