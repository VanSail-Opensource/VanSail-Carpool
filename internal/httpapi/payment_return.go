package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
)

func validReturnProvider(provider string) bool { return provider == "ezpay" || provider == "bepusdt" }

// Navigation forwards an existing checkout capability; it never issues one.
func (s *Server) paymentReturn(w http.ResponseWriter, r *http.Request) {
	provider := chiURLParam(r, "provider")
	if !validReturnProvider(provider) {
		s.writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Payment return route not found.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	orderID := r.URL.Query().Get("order")
	if pathOrderID := chiURLParam(r, "orderID"); pathOrderID != "" {
		orderID = pathOrderID
	}
	query := url.Values{"paymentOrder": {orderID}, "provider": {provider}}
	capability := r.URL.Query().Get("capability")
	if billing.ValidPaymentReturnCapability(s.deps.RequestSigningKey, provider, orderID, capability, time.Now().UTC()) {
		query.Set("paymentCapability", capability)
	}
	target := *s.deps.PublicURL
	target.Path = strings.TrimRight(target.Path, "/") + "/payment-result"
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

func (s *Server) paymentReturnStatus(w http.ResponseWriter, r *http.Request) {
	provider := chiURLParam(r, "provider")
	if !validReturnProvider(provider) {
		s.writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Payment return route not found.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	orderID := chiURLParam(r, "orderID")
	if !billing.ValidPaymentReturnCapability(s.deps.RequestSigningKey, provider, orderID, r.URL.Query().Get("capability"), time.Now().UTC()) {
		// Legacy and expired links require the same signed owner identity as an
		// ordinary receipt read. Navigation can never renew or upgrade a link.
		s.requireSignedRequest(s.requireSession(http.HandlerFunc(s.paymentReturnOwnerStatus))).ServeHTTP(w, r)
		return
	}
	details, err := s.deps.Billing.ReturnDetails(r.Context(), provider, orderID)
	if err != nil {
		s.paymentReadError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (s *Server) paymentReturnOwnerStatus(w http.ResponseWriter, r *http.Request) {
	details, err := s.deps.Billing.ReturnDetailsForOwner(r.Context(), chiURLParam(r, "provider"), chiURLParam(r, "orderID"), currentUser(r).ID)
	if err != nil {
		s.paymentReadError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}
