package httpapi

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) xnodeQueueCommand(w http.ResponseWriter, r *http.Request) {
	var input xnode.CommandRequest
	if err := decodeJSON(w, r, &input); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	receipt, err := s.deps.XNode.QueueCommand(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}
func (s *Server) xnodeDeleteNode(w http.ResponseWriter, r *http.Request) {
	s.xnodeDeleteResource(w, r, s.deps.XNode.DeleteNode)
}
func (s *Server) xnodeDeleteGroup(w http.ResponseWriter, r *http.Request) {
	s.xnodeDeleteResource(w, r, s.deps.XNode.DeleteGroup)
}
func (s *Server) xnodeDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	s.xnodeDeleteResource(w, r, s.deps.XNode.DeleteEndpoint)
}
func (s *Server) xnodeDeleteResource(w http.ResponseWriter, r *http.Request, remove func(context.Context, string) error) {
	if err := remove(r.Context(), chiURLParam(r, "id")); err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
