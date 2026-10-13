package httpapi

import (
	"errors"
	"net/http"

	"github.com/txyyddss/Remna-User-Panel/internal/integrations/turnstile"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

func (s *Server) panelAuthState(r *http.Request, user model.User, entered bool) (authState, error) {
	state := authState{Authenticated: true, User: mapUser(user)}
	if browserScopedRequest(r) {
		return state, nil
	}
	if s.deps.FirstEntry == nil {
		return state, nil
	}
	captcha, err := s.deps.FirstEntry.State(r.Context(), user, entered)
	state.Captcha = &captcha
	return state, err
}

func (s *Server) requireCaptcha(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if browserScopedRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if s.deps.FirstEntry == nil {
			next.ServeHTTP(w, r)
			return
		}
		switch r.URL.Path {
		case "/api/v1/me", "/api/v1/me/captcha", "/api/v1/me/display-currency":
			next.ServeHTTP(w, r)
			return
		}
		state, err := s.deps.FirstEntry.State(r.Context(), currentUser(r), false)
		if err != nil {
			s.captchaFailure(w, r, err)
			return
		}
		if state.Required {
			s.writeError(w, r, http.StatusForbidden, "CAPTCHA_REQUIRED", "Complete the panel entry verification to continue.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) verifyCaptcha(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &request); err != nil || len(request.Token) == 0 || len(request.Token) > 2048 {
		s.captchaFailure(w, r, turnstile.ErrRejected)
		return
	}
	if s.deps.FirstEntry == nil {
		s.captchaFailure(w, r, turnstile.ErrUnavailable)
		return
	}
	if err := s.deps.FirstEntry.Verify(r.Context(), currentUser(r), request.Token); err != nil {
		s.captchaFailure(w, r, err)
		return
	}
	state, err := s.panelAuthState(r, currentUser(r), false)
	if err != nil {
		s.captchaFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) captchaFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, turnstile.ErrRejected) {
		s.writeError(w, r, http.StatusUnprocessableEntity, "CAPTCHA_REJECTED", "Verification expired or failed. Please solve a fresh challenge.")
		return
	}
	s.writeError(w, r, http.StatusServiceUnavailable, "CAPTCHA_UNAVAILABLE", "Verification is temporarily unavailable. Please retry.")
}
