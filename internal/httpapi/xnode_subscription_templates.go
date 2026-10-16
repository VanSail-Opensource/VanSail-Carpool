package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) xnodeSubscriptionTemplates(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNode.SubscriptionTemplates(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeSaveSubscriptionTemplates(w http.ResponseWriter, r *http.Request) {
	var input xnode.SubscriptionTemplateConfig
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	value, err := s.deps.XNode.SaveSubscriptionTemplates(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodePreviewSubscriptionTemplate(w http.ResponseWriter, r *http.Request) {
	var input subscription.Template
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	raw, validation, err := s.deps.XNode.PreviewSubscriptionTemplate(input)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]any{"validation": validation, "preview": string(raw)})
}
