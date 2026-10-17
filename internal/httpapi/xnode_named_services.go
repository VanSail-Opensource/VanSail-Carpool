package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) xnodeNamedServices(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.XNode.NamedServices(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) xnodeSaveNamedService(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name   string              `json:"name"`
		Access xnode.ServiceAccess `json:"access"`
	}
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	item, err := s.deps.XNode.SaveNamedService(r.Context(), chiURLParam(r, "id"), input.Name, input.Access)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) xnodeRotateServiceSubscription(w http.ResponseWriter, r *http.Request) {
	identity, err := s.deps.XNode.Identity(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	access, err := s.deps.XNode.ServiceAccess(r.Context(), identity)
	if err != nil || access == nil {
		s.xnodeError(w, r, xnode.ErrNotFound)
		return
	}
	url, err := s.deps.XNode.RevokeSubscription(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}
func (s *Server) xnodeServiceRouting(w http.ResponseWriter, r *http.Request) {
	owner := "native." + chiURLParam(r, "id")
	item, err := s.deps.XNode.OwnedCustomization(r.Context(), owner, chiURLParam(r, "nodeId"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) xnodeSaveServiceRouting(w http.ResponseWriter, r *http.Request) {
	var input xnode.Customization
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	part := r.URL.Query().Get("part")
	if part != "" && part != "rules" && part != "outbounds" {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	owner := "native." + chiURLParam(r, "id")
	node := chiURLParam(r, "nodeId")
	var item xnode.Customization
	var err error
	if r.URL.Query().Get("part") == "outbounds" {
		item, err = s.deps.XNode.SaveOwnedOutbounds(r.Context(), owner, node, input.Revision, input.Outbounds)
	} else {
		item, err = s.deps.XNode.SaveOwnedRouting(r.Context(), owner, node, input.Revision, input.Routing)
	}
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, item)
}
