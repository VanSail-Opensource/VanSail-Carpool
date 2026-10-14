package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/browserauth"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
	"github.com/go-chi/chi/v5"
)

const browserSessionCookie = "txc_admin_session"
const browserRequestKeyCookie = "txc_admin_request_key"

func (s *Server) mountBrowserAuth(router chi.Router) {
	router.Get("/api/v1/browser/config", s.browserConfig)
	router.Get("/api/v1/auth/browser/session", s.browserSession)
	router.With(s.limitAuthentication, s.requireBrowserOrigin).Post("/api/v1/auth/browser/exchange", s.browserExchange)
	router.With(s.requireBrowserOrigin, s.requireSignedRequest, s.requireSession).Post("/api/v1/auth/browser/session/renew", s.browserRenew)
	router.With(s.requireBrowserOrigin, s.requireSignedRequest, s.requireSession).Post("/api/v1/auth/browser/logout", s.browserLogout)
	router.Group(func(access chi.Router) {
		access.Use(s.requireSignedRequest, s.requireSession, s.requireAdmin)
		access.Post("/api/v1/admin-access/links", s.browserLink)
	})
}

func browserScopedRequest(r *http.Request) bool {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/admin/") || strings.HasPrefix(path, "/api/v1/auth/browser/") {
		return true
	}
	return r.Header.Get("X-TXC-Session-Scope") == "browser" && (path == "/api/v1/me" || path == "/api/v1/me/display-currency" || strings.HasPrefix(path, "/api/v1/admin-access/") || path == "/api/v1/ip-lookup" || strings.HasPrefix(path, "/api/v1/ip-lookup/"))
}

func requestCookieNames(r *http.Request) (string, string) {
	if browserScopedRequest(r) {
		return browserSessionCookie, browserRequestKeyCookie
	}
	return sessionCookie, requestauth.ClientKeyCookie
}

func (s *Server) requireBrowserOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin, err := url.Parse(strings.TrimSpace(r.Header.Get("Origin")))
		if err != nil || !sameOrigin(origin, s.deps.PublicURL) || (r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin") {
			s.writeError(w, r, http.StatusForbidden, "BROWSER_ORIGIN_REQUIRED", "This action must originate from this site.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) miniAppURL() string {
	if s.deps.Affiliates == nil {
		return "https://t.me/"
	}
	username := strings.TrimPrefix(s.deps.Affiliates.BotUsername(), "@")
	if username == "" {
		return "https://t.me/"
	}
	return "https://t.me/" + url.PathEscape(username) + "?startapp"
}

func (s *Server) browserConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"miniAppUrl": s.miniAppURL()})
}

func (s *Server) browserFailure(w http.ResponseWriter, r *http.Request, status int, code string) {
	s.writeError(w, r, status, code, "Browser access could not be completed. Please sign in again.")
}

func (s *Server) browserSessionFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, browserauth.ErrUnauthorized) {
		s.browserFailure(w, r, http.StatusUnauthorized, "UNAUTHENTICATED")
		return
	}
	s.browserFailure(w, r, http.StatusServiceUnavailable, "BROWSER_SESSION_UNAVAILABLE")
}
