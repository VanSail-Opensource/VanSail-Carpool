package httpapi

import (
	"context"
	"net/http"
)

func (s *Server) requireBrowserSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(browserSessionCookie)
		if err != nil {
			s.browserFailure(w, r, 401, "UNAUTHENTICATED")
			return
		}
		user, expires, renewed, err := s.deps.BrowserAuth.Renew(r.Context(), cookie.Value)
		if err != nil {
			s.browserSessionFailure(w, r, err)
			return
		}
		if renewed {
			if err = s.setBrowserCookies(w, cookie.Value, expires); err != nil {
				s.browserFailure(w, r, 500, "SESSION_RENEW_FAILED")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}
