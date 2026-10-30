package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
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
func (s *Server) xnodeFirewallQuote(w http.ResponseWriter, r *http.Request) {
	quote, err := s.deps.XNode.OwnedFirewallQuote(r.Context(), currentUser(r).ID)
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
