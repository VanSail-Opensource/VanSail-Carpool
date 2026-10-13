package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/browserauth"
)

func (s *Server) setEncryptedBrowserCookie(w http.ResponseWriter, name string, value any, expires time.Time) error {
	encoded, err := s.deps.BrowserAuth.SealCookie(name, value)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: encoded, Path: "/", MaxAge: int(time.Until(expires).Seconds()), Expires: expires, Secure: s.deps.SecureCookies, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return nil
}

func (s *Server) browserOAuthStart(w http.ResponseWriter, r *http.Request) {
	flow, destination, err := s.deps.BrowserAuth.StartOAuth(r.Context(), r.URL.Query().Get("returnTo"))
	if err != nil {
		s.browserFailure(w, r, http.StatusServiceUnavailable, "TELEGRAM_OAUTH_UNAVAILABLE")
		return
	}
	if err = s.setEncryptedBrowserCookie(w, oauthFlowCookie, flow, flow.Expires); err != nil {
		s.browserFailure(w, r, 500, "OAUTH_STATE_FAILED")
		return
	}
	http.Redirect(w, r, destination, http.StatusFound)
}

func (s *Server) browserOAuthCallback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(oauthFlowCookie)
	s.clearBrowserCookie(w, oauthFlowCookie)
	var flow browserauth.OAuthFlow
	if err != nil || s.deps.BrowserAuth.OpenCookie(oauthFlowCookie, cookie.Value, &flow) != nil || !flow.Expires.After(time.Now()) || subtle.ConstantTimeCompare([]byte(flow.State), []byte(r.URL.Query().Get("state"))) != 1 || !browserauth.ValidDestination(flow.ReturnTo) {
		http.Redirect(w, r, "/admin/login?error=OAUTH_STATE_INVALID", http.StatusSeeOther)
		return
	}
	if providerError := r.URL.Query().Get("error"); providerError != "" {
		errorCode := "OAUTH_PROVIDER_REJECTED"
		if providerError == "access_denied" {
			errorCode = "OAUTH_CANCELLED"
		}
		http.Redirect(w, r, "/admin/login?error="+errorCode, http.StatusSeeOther)
		return
	}
	profile, err := s.deps.BrowserAuth.CompleteOAuth(r.Context(), flow, r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		errorCode := "OAUTH_LOGIN_FAILED"
		switch {
		case errors.Is(err, browserauth.ErrProviderUnavailable):
			errorCode = "OAUTH_PROVIDER_UNAVAILABLE"
		case errors.Is(err, browserauth.ErrProviderRejected):
			errorCode = "OAUTH_PROVIDER_REJECTED"
		case errors.Is(err, browserauth.ErrInvalidOAuthResult):
			errorCode = "OAUTH_RESPONSE_INVALID"
		}
		http.Redirect(w, r, "/admin/login?error="+errorCode, http.StatusSeeOther)
		return
	}
	_, token, expires, err := s.deps.BrowserAuth.CreateSession(r.Context(), profile)
	if err == browserauth.ErrUnauthorized {
		member := browserMemberState{Profile: profile, Expires: time.Now().UTC().Add(5 * time.Minute)}
		if err = s.setEncryptedBrowserCookie(w, browserMemberCookie, member, member.Expires); err != nil {
			s.browserFailure(w, r, 500, "OAUTH_STATE_FAILED")
			return
		}
		// Signing in as a member cannot retain an earlier device's admin identity.
		if previous, cookieErr := r.Cookie(browserSessionCookie); cookieErr == nil {
			if err = s.deps.BrowserAuth.Logout(r.Context(), previous.Value); err != nil {
				s.browserFailure(w, r, 500, "SESSION_REVOKE_FAILED")
				return
			}
		}
		s.clearBrowserCookie(w, browserSessionCookie)
		s.clearBrowserCookie(w, browserRequestKeyCookie)
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		s.browserFailure(w, r, 500, "SESSION_CREATE_FAILED")
		return
	}
	if err = s.setBrowserCookies(w, token, expires); err != nil {
		s.browserFailure(w, r, 500, "SESSION_CREATE_FAILED")
		return
	}
	s.clearBrowserCookie(w, browserMemberCookie)
	http.Redirect(w, r, flow.ReturnTo, http.StatusSeeOther)
}

func (s *Server) oauthSettingsResponse(config browserauth.Config) map[string]any {
	return map[string]any{"enabled": config.Enabled, "clientId": config.ClientID, "clientSecretConfigured": config.ClientSecret != "", "callbackUrl": s.deps.BrowserAuth.CallbackURL()}
}

func (s *Server) browserOAuthSettings(w http.ResponseWriter, r *http.Request) {
	config, err := s.deps.BrowserAuth.Config(r.Context())
	if err != nil {
		s.browserFailure(w, r, 500, "OAUTH_CONFIG_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, s.oauthSettingsResponse(config))
}

func (s *Server) browserSaveOAuthSettings(w http.ResponseWriter, r *http.Request) {
	var config browserauth.Config
	if err := decodeJSON(w, r, &config); err != nil {
		s.browserFailure(w, r, 400, "INVALID_REQUEST")
		return
	}
	saved, err := s.deps.BrowserAuth.SaveConfig(r.Context(), currentUser(r).ID, config)
	if err != nil {
		s.writeError(w, r, 422, "OAUTH_CONFIG_INVALID", "Check the Telegram OAuth client ID and secret before enabling login.")
		return
	}
	writeJSON(w, http.StatusOK, s.oauthSettingsResponse(saved))
}

func (s *Server) browserLink(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ReturnTo string `json:"returnTo"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.browserFailure(w, r, 400, "INVALID_REQUEST")
		return
	}
	link, expires, err := s.deps.BrowserAuth.Link(r.Context(), currentUser(r), input.ReturnTo)
	if err != nil {
		s.browserFailure(w, r, 422, "BROWSER_LINK_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"url": link, "expiresAt": expires})
}
