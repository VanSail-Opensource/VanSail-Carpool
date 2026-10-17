package httpapi

import (
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func (s *Server) mountXNodeMembers(router chi.Router) {
	if s.deps.XNode == nil {
		return
	}
	router.Route("/api/v1/xnode", func(router chi.Router) {
		router.Get("/firewall/exemption", s.xnodeFirewallQuote)
		router.Post("/firewall/exemption", s.xnodeFirewallPurchase)
		router.Get("/metrics", s.xnodeMemberMetrics)
		router.Get("/subscription", s.xnodeOwnedSubscription)
		router.Get("/routing/templates", s.xnodeRoutingTemplates)
		router.Get("/routing/{id}", s.xnodeMemberCustomization)
		router.Put("/routing/{id}/outbounds", s.xnodeMemberOutbounds)
		router.Put("/routing/{id}/rules", s.xnodeMemberRouting)
	})
}
func (s *Server) xnodeMemberMetrics(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNodeMaster.UserMetrics(r.Context(), currentUser(r).ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeOwnedSubscription(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNode.OwnedSubscription(r.Context(), currentUser(r).ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeRoutingTemplates(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNode.RoutingTemplates(r.Context(), currentUser(r).ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]any{"items": value})
}
func (s *Server) xnodeMemberCustomization(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNode.OwnedCustomization(r.Context(), currentUser(r).ID, chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeMemberOutbounds(w http.ResponseWriter, r *http.Request) {
	s.saveXNodeCustomization(w, r, false)
}
func (s *Server) xnodeMemberRouting(w http.ResponseWriter, r *http.Request) {
	s.saveXNodeCustomization(w, r, true)
}
func (s *Server) saveXNodeCustomization(w http.ResponseWriter, r *http.Request, rules bool) {
	var input struct {
		Revision int64           `json:"revision"`
		Objects  json.RawMessage `json:"objects"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	var value xnode.Customization
	var err error
	if rules {
		value, err = s.deps.XNode.SaveOwnedRouting(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), input.Revision, input.Objects)
	} else {
		value, err = s.deps.XNode.SaveOwnedOutbounds(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), input.Revision, input.Objects)
	}
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func privateSubscriptionHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
func (s *Server) xnodeBearerMetadata(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	value, err := s.deps.XNode.BearerMetadata(r.Context(), chiURLParam(r, "token"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeBearerSubscription(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	format := subscriptionContentFormat(r)
	raw, metadata, err := s.deps.XNode.BearerSubscription(r.Context(), chiURLParam(r, "token"), format)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", map[string]string{"native": "text/plain; charset=utf-8", "xray": "application/json", "sing-box": "application/json", "clash": "text/yaml; charset=utf-8"}[format])
	expire := int64(0)
	if metadata.ExpiresAt != nil {
		expire = metadata.ExpiresAt.Unix()
	}
	w.Header().Set("Subscription-Userinfo", "upload=0; download="+metadata.UsedBytes+"; total="+metadata.LimitBytes+"; expire="+strconv.FormatInt(expire, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
