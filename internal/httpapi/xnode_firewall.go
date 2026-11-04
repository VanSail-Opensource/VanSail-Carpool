package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
	"strconv"
)

func (s *Server) xnodeFirewallBans(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil {
			s.xnodeError(w, r, xnode.ErrInvalid)
			return
		}
	}
	page, err := s.deps.XNode.SearchFirewallBans(r.Context(), r.URL.Query().Get("query"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) xnodeFirewallRecords(w http.ResponseWriter, r *http.Request) {
	var request xnode.FirewallRecordSearch
	if err := decodeJSON(w, r, &request); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	page, err := s.deps.XNode.SearchFirewallRecords(r.Context(), request)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, page)
}
