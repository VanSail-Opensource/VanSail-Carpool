package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
)

func (s *Server) requireSignedRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Authorization")) > 0 {
			s.authenticateAPIToken(w, r, next)
			return
		}
		if isStreamingBackupUpload(r) {
			if !s.authorizeStreamingBackupUpload(w, r) {
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		sessionName, keyName := requestCookieNames(r)
		session, sessionErr := r.Cookie(sessionName)
		clientKey, keyErr := r.Cookie(keyName)
		if sessionErr != nil || keyErr != nil {
			s.writeError(w, r, http.StatusUnauthorized, "REQUEST_SIGNATURE_REQUIRED", "Open VanSail Carpool from Telegram to refresh your secure session.")
			return
		}
		if err := s.requests.Verify(r, session.Value, clientKey.Value); err != nil {
			s.writeRequestAuthError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeRequestAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, requestauth.ErrBodyTooLarge):
		s.writeError(w, r, http.StatusRequestEntityTooLarge, "REQUEST_BODY_TOO_LARGE", "The request body is too large.")
	case errors.Is(err, requestauth.ErrReplay):
		s.writeError(w, r, http.StatusConflict, "REQUEST_REPLAYED", "This request has already been received.")
	case errors.Is(err, requestauth.ErrRateLimited):
		w.Header().Set("Retry-After", "30")
		s.writeError(w, r, http.StatusTooManyRequests, "REQUEST_RATE_LIMITED", "This session is sending requests too quickly.")
	case errors.Is(err, requestauth.ErrStale):
		s.writeError(w, r, http.StatusUnauthorized, "REQUEST_SIGNATURE_STALE", "Refresh the app and retry this request.")
	case errors.Is(err, requestauth.ErrRequired), errors.Is(err, requestauth.ErrMalformed):
		s.writeError(w, r, http.StatusUnauthorized, "REQUEST_SIGNATURE_REQUIRED", "Open VanSail Carpool from Telegram to refresh your secure session.")
	default:
		s.writeError(w, r, http.StatusUnauthorized, "REQUEST_SIGNATURE_INVALID", "Open VanSail Carpool from Telegram to refresh your secure session.")
	}
}
