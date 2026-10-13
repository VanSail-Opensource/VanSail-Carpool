package httpapi

import (
	"errors"
	"github.com/txyyddss/Remna-User-Panel/internal/browserauth"
	"net/http"
	"strconv"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/requestauth"
)

type browserMemberState struct {
	Profile model.TelegramProfile `json:"profile"`
	Expires time.Time             `json:"expires"`
}

func (s *Server) setBrowserCookies(w http.ResponseWriter, token string, expires time.Time) error {
	key, err := s.requests.ClientKey(token)
	if err != nil {
		return err
	}
	common := http.Cookie{Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), Secure: s.deps.SecureCookies, SameSite: http.SameSiteLaxMode}
	session := common
	session.Name, session.Value, session.HttpOnly = browserSessionCookie, token, true
	http.SetCookie(w, &session)
	signing := common
	signing.Name, signing.Value = browserRequestKeyCookie, key
	http.SetCookie(w, &signing)
	return nil
}

func (s *Server) clearBrowserCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: name != browserRequestKeyCookie, Secure: s.deps.SecureCookies, SameSite: http.SameSiteLaxMode})
}

func browserAdminState(user model.User, expires time.Time, returnTo string) map[string]any {
	state := map[string]any{"authenticated": true, "kind": "admin", "user": mapUser(user), "expiresAt": expires}
	if returnTo != "" {
		state["returnTo"] = returnTo
	}
	return state
}

func (s *Server) writeBrowserMember(w http.ResponseWriter, profile model.TelegramProfile) {
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "kind": "member", "miniAppUrl": s.miniAppURL(), "user": map[string]string{"telegramId": strconv.FormatInt(profile.ID, 10), "firstName": profile.FirstName, "lastName": profile.LastName, "telegramUsername": profile.Username}})
}

func (s *Server) verifyBrowserSessionRead(w http.ResponseWriter, r *http.Request, sessionName, keyName string) (*http.Cookie, bool) {
	session, err := r.Cookie(sessionName)
	if err != nil {
		return nil, false
	}
	key, err := r.Cookie(keyName)
	if err != nil {
		s.browserFailure(w, r, http.StatusUnauthorized, "REQUEST_SIGNATURE_REQUIRED")
		return nil, true
	}
	if err = s.requests.Verify(r, session.Value, key.Value); err != nil {
		s.writeRequestAuthError(w, r, err)
		return nil, true
	}
	return session, false
}

func (s *Server) browserSession(w http.ResponseWriter, r *http.Request) {
	session, failed := s.verifyBrowserSessionRead(w, r, browserSessionCookie, browserRequestKeyCookie)
	if failed {
		return
	}
	if session != nil {
		user, expires, renewed, err := s.deps.BrowserAuth.Renew(r.Context(), session.Value)
		if err == nil {
			if renewed {
				if err = s.setBrowserCookies(w, session.Value, expires); err != nil {
					s.browserFailure(w, r, 500, "SESSION_CREATE_FAILED")
					return
				}
			}
			writeJSON(w, http.StatusOK, browserAdminState(user, expires, ""))
			return
		}
		if !errors.Is(err, browserauth.ErrUnauthorized) {
			s.browserSessionFailure(w, r, err)
			return
		}
		s.clearBrowserCookie(w, browserSessionCookie)
		s.clearBrowserCookie(w, browserRequestKeyCookie)
	}
	if cookie, err := r.Cookie(browserMemberCookie); err == nil {
		var member browserMemberState
		if s.deps.BrowserAuth.OpenCookie(browserMemberCookie, cookie.Value, &member) == nil && member.Expires.After(time.Now()) && member.Profile.ID > 0 {
			s.writeBrowserMember(w, member.Profile)
			return
		}
		s.clearBrowserCookie(w, browserMemberCookie)
	}
	if session != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "kind": "anonymous", "miniAppUrl": s.miniAppURL()})
		return
	}
	miniSession, failed := s.verifyBrowserSessionRead(w, r, sessionCookie, requestauth.ClientKeyCookie)
	if failed {
		return
	}
	if miniSession != nil {
		user, err := s.deps.Accounts.UserBySession(r.Context(), miniSession.Value)
		if err == nil && (user.Role != "admin" || !s.isAdminTelegramID(user.TelegramID)) {
			s.writeBrowserMember(w, model.TelegramProfile{ID: user.TelegramID, FirstName: user.TelegramFirstName, LastName: user.TelegramLastName, Username: user.TelegramUsername})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "kind": "anonymous", "miniAppUrl": s.miniAppURL()})
}

func (s *Server) browserExchange(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.browserFailure(w, r, 400, "INVALID_REQUEST")
		return
	}
	user, token, expires, returnTo, err := s.deps.BrowserAuth.Exchange(r.Context(), input.Token)
	if err != nil {
		s.browserFailure(w, r, 401, "BROWSER_LINK_INVALID")
		return
	}
	if err = s.setBrowserCookies(w, token, expires); err != nil {
		s.browserFailure(w, r, 500, "SESSION_CREATE_FAILED")
		return
	}
	s.clearBrowserCookie(w, browserMemberCookie)
	writeJSON(w, http.StatusOK, browserAdminState(user, expires, returnTo))
}

func (s *Server) browserRenew(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(browserSessionCookie)
	if err != nil {
		s.browserFailure(w, r, 401, "UNAUTHENTICATED")
		return
	}
	user, expires, _, err := s.deps.BrowserAuth.Renew(r.Context(), cookie.Value)
	if err != nil {
		s.browserSessionFailure(w, r, err)
		return
	}
	if err = s.setBrowserCookies(w, cookie.Value, expires); err != nil {
		s.browserFailure(w, r, 500, "SESSION_CREATE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, browserAdminState(user, expires, ""))
}

func (s *Server) browserLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(browserSessionCookie)
	if err != nil {
		s.browserFailure(w, r, 401, "UNAUTHENTICATED")
		return
	}
	if err = s.deps.BrowserAuth.Logout(r.Context(), cookie.Value); err != nil {
		s.browserFailure(w, r, 500, "SESSION_REVOKE_FAILED")
		return
	}
	s.clearBrowserCookie(w, browserSessionCookie)
	s.clearBrowserCookie(w, browserRequestKeyCookie)
	s.clearBrowserCookie(w, browserMemberCookie)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "kind": "anonymous", "miniAppUrl": s.miniAppURL()})
}
