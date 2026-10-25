package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) xnodeNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.deps.XNode.Nodes(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": nodes})
}
func (s *Server) xnodeNode(w http.ResponseWriter, r *http.Request) {
	node, settings, err := s.deps.XNode.Node(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"node": node, "settings": settings})
}
func (s *Server) xnodeSaveNode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Node     xnode.Node         `json:"node"`
		Settings xnode.NodeSettings `json:"settings"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if id := chiURLParam(r, "id"); id != "" {
		input.Node.ID = id
	}
	node, err := s.deps.XNode.SaveNodeAs(r.Context(), input.Node, input.Settings, currentUser(r).ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}
func (s *Server) xnodeEnrollment(w http.ResponseWriter, r *http.Request) {
	url, token, err := s.deps.XNode.Enrollment(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]string{"masterUrl": url, "token": token})
}
func (s *Server) xnodeGroups(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.XNode.Groups(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) xnodeSaveGroup(w http.ResponseWriter, r *http.Request) {
	var item xnode.Group
	if err := decodeJSON(w, r, &item); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if id := chiURLParam(r, "id"); id != "" {
		item.ID = id
	}
	item, err := s.deps.XNode.SaveGroup(r.Context(), item)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) xnodeEndpoints(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.XNode.Endpoints(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) xnodeSaveEndpoint(w http.ResponseWriter, r *http.Request) {
	var item xnode.Endpoint
	if err := decodeJSON(w, r, &item); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if id := chiURLParam(r, "id"); id != "" {
		item.ID = id
	}
	item, err := s.deps.XNode.SaveEndpoint(r.Context(), item)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
