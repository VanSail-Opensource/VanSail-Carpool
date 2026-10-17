package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) xnodeFirewallBans(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.XNode.FirewallBans(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) xnodeFirewallQuote(w http.ResponseWriter, r *http.Request) {
	quote, err := s.deps.XNode.FirewallQuote(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, quote)
}
func (s *Server) xnodeFirewallPurchase(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input xnode.FirewallPurchase
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	input.Key = key
	grant, err := s.deps.XNode.PurchaseFirewallExemption(r.Context(), user.ID, input)
	if errors.Is(err, database.ErrInsufficientBalance) {
		s.writeError(w, r, http.StatusConflict, "INSUFFICIENT_BALANCE", "Insufficient TXB balance.")
		return
	}
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusAccepted, grant)
}
