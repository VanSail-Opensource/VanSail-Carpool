package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func (s *Server) mountXNodeMembers(router chi.Router) {
	if s.deps.XNode == nil {
		return
	}
	router.Get("/api/v1/payg", s.ownedPAYG)
	router.Route("/api/v1/xnode", func(router chi.Router) {
		router.Get("/diy", s.xnodeDIY)
		router.Post("/diy/preview", s.xnodePreviewDIY)
		router.Put("/diy", s.xnodeSaveDIY)
		router.Get("/firewall/exemption", s.xnodeFirewallQuote)
		router.Post("/firewall/exemption", s.xnodeFirewallPurchase)
		router.Get("/metrics", s.xnodeMemberMetrics)
		router.Get("/subscription", s.xnodeOwnedSubscription)
		router.Get("/routing", s.xnodeRoutingPlan)
		router.Put("/routing", s.xnodeSaveRoutingPlan)
		router.Post("/routing/preview", s.xnodePreviewRoutingPlan)
		router.Post("/routing/speedtests", s.xnodeStartRouteSpeedtest)
		router.Get("/routing/speedtests/{id}", s.xnodeRouteSpeedtestResult)
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
	raw, metadata, err := s.deps.XNode.BearerSubscriptionForAgent(r.Context(), chiURLParam(r, "token"), r.URL.Query().Get("format"), r.UserAgent())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.Header().Set("X-Subscription-Format", metadata.SelectedFormat)
	w.Header().Add("Vary", "User-Agent")
	if s.subscriptionSpecialResponse(w, r, metadata) {
		return
	}
	w.Header().Set("Content-Type", map[string]string{"base64": "text/plain; charset=utf-8", "stash": "text/yaml; charset=utf-8", "native": "text/plain; charset=utf-8", "xray": "application/json", "sing-box": "application/json", "clash": "text/yaml; charset=utf-8"}[metadata.SelectedFormat])
	expire := int64(0)
	if metadata.ExpiresAt != nil {
		expire = metadata.ExpiresAt.Unix()
	}
	w.Header().Set("Subscription-Userinfo", "upload=0; download="+metadata.UsedBytes+"; total="+metadata.LimitBytes+"; expire="+strconv.FormatInt(expire, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
