package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
	"strings"
)

func (s *Server) xnodeRoutingPlan(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNode.OwnedRoutingPlan(r.Context(), currentUser(r).ID)
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeSaveRoutingPlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision int64                 `json:"revision"`
		Choices  []xnode.RoutingChoice `json:"choices"`
	}
	if decodeJSON(w, r, &input) != nil || input.Choices == nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.XNode.QueueOwnedRouting(r.Context(), currentUser(r).ID, key, input.Revision, input.Choices)
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusAccepted, receipt)
}
func (s *Server) xnodePreviewRoutingPlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Choices     []xnode.RoutingChoice    `json:"choices"`
		Start       string                   `json:"start"`
		Destination routingmatch.Destination `json:"destination"`
	}
	if decodeJSON(w, r, &input) != nil || input.Choices == nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	paths, trace, err := s.deps.XNode.PreviewOwnedRouting(r.Context(), currentUser(r).ID, input.Choices, input.Start, input.Destination)
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]any{"paths": paths, "trace": trace})
}
func (s *Server) xnodeRoutingTemplateConfig(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNode.RoutingTemplateConfig(r.Context())
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeSaveRoutingTemplateConfig(w http.ResponseWriter, r *http.Request) {
	var input xnode.RoutingTemplateConfig
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	value, err := s.deps.XNode.SaveRoutingTemplates(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.xnodeRoutingError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeRoutingError(w http.ResponseWriter, r *http.Request, err error) {
	var d *routingmatch.Diagnostic
	if errors.As(err, &d) {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: d.Code, Message: "The routing conditions could not be accepted.", Details: map[string]string{"rules": strings.Join(d.Rules, ",")}, RequestID: middleware.GetReqID(r.Context())})
		return
	}
	s.xnodeError(w, r, err)
}
