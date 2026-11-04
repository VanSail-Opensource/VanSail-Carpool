package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
	"strconv"
)

func (s *Server) memberConnections(w http.ResponseWriter, r *http.Request) {
	page, size := 1, 25
	var err error
	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
	}
	if err == nil {
		if raw := r.URL.Query().Get("pageSize"); raw != "" {
			size, err = strconv.Atoi(raw)
		}
	}
	if err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if s.deps.XNode == nil {
		s.xnodeError(w, r, xnode.ErrUnavailable)
		return
	}
	result, err := s.deps.XNode.OwnedConnections(r.Context(), currentUser(r).ID, r.URL.Query().Get("search"), page, size)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) grantMemberConnection(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var input struct {
		IP string `json:"ip"`
	}
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if s.deps.XNode == nil {
		s.xnodeError(w, r, xnode.ErrUnavailable)
		return
	}
	result, err := s.deps.XNode.GrantOwnedConnection(r.Context(), user.ID, input.IP)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}
