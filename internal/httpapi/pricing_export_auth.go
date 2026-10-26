package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type pricingExportGrant struct {
	actor          string
	session, scope [32]byte
	expires        time.Time
}
type pricingExportAuthorization struct {
	mu     sync.Mutex
	grants map[[32]byte]pricingExportGrant
}

func exportScopeHash(scope database.PricingExportScope) [32]byte {
	raw, _ := json.Marshal(scope)
	return sha256.Sum256(raw)
}
func exportSessionHash(r *http.Request) [32]byte {
	c, err := r.Cookie(browserSessionCookie)
	if err != nil {
		return [32]byte{}
	}
	return sha256.Sum256([]byte(c.Value))
}

func (s *Server) pricingExportAuthorization(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if apiPrincipal(r) != nil || exportSessionHash(r) == ([32]byte{}) {
		s.writeError(w, r, http.StatusForbidden, "PRICING_EXPORT_AUTHORIZATION_REQUIRED", "Sensitive exports require the signed administrator browser session.")
		return
	}
	if _, ok := s.adminTelegramIDs[user.TelegramID]; !ok {
		s.writeError(w, r, http.StatusForbidden, "PRICING_EXPORT_AUTHORIZATION_REQUIRED", "Additional export authorization is required.")
		return
	}
	var input struct {
		Scope     database.PricingExportScope `json:"scope"`
		Reason    string                      `json:"reason"`
		Authorize bool                        `json:"authorize"`
	}
	if err := decodeJSON(w, r, &input); err != nil || !input.Authorize || input.Reason == "" || len(input.Reason) > 1000 || input.Scope.Validate() != nil {
		s.writeError(w, r, http.StatusBadRequest, "PRICING_EXPORT_AUTHORIZATION_REQUIRED", "Explicit scoped export authorization and a reason are required.")
		return
	}
	now := time.Now().UTC()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		s.pricingError(w, r, err)
		return
	}
	token := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	if err := s.deps.Store.AppendAudit(r.Context(), &user.ID, "pricing.export.authorize", "pricing", "export", input.Reason, now); err != nil {
		s.pricingError(w, r, err)
		return
	}
	s.pricingExportAuth.mu.Lock()
	defer s.pricingExportAuth.mu.Unlock()
	if s.pricingExportAuth.grants == nil {
		s.pricingExportAuth.grants = map[[32]byte]pricingExportGrant{}
	}
	for k, g := range s.pricingExportAuth.grants {
		if !g.expires.After(now) {
			delete(s.pricingExportAuth.grants, k)
		}
	}
	if len(s.pricingExportAuth.grants) >= 128 {
		s.writeError(w, r, http.StatusTooManyRequests, "PRICING_EXPORT_RESOURCE_LIMIT", "Export authorization limit reached.")
		return
	}
	expires := now.Add(5 * time.Minute)
	s.pricingExportAuth.grants[hash] = pricingExportGrant{actor: user.ID, session: exportSessionHash(r), scope: exportScopeHash(input.Scope), expires: expires}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expiresAt": expires})
}

func (s *Server) consumePricingExportGrant(r *http.Request, scope database.PricingExportScope, token string) bool {
	hash := sha256.Sum256([]byte(token))
	s.pricingExportAuth.mu.Lock()
	defer s.pricingExportAuth.mu.Unlock()
	g, ok := s.pricingExportAuth.grants[hash]
	if !ok || !g.expires.After(time.Now()) || g.actor != currentUser(r).ID || g.session != exportSessionHash(r) || g.scope != exportScopeHash(scope) {
		return false
	}
	delete(s.pricingExportAuth.grants, hash)
	return true
}
