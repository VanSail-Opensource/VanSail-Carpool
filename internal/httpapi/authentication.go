package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/currencydisplay"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
)

type userResponse struct {
	ID                string     `json:"id"`
	TelegramID        string     `json:"telegramId"`
	FirstName         string     `json:"firstName"`
	LastName          string     `json:"lastName"`
	TelegramUsername  string     `json:"telegramUsername"`
	DisplayName       string     `json:"displayName"`
	Username          *string    `json:"username"`
	Role              string     `json:"role"`
	OnboardingState   string     `json:"onboardingState"`
	GroupJoined       bool       `json:"groupJoined"`
	ChannelJoined     bool       `json:"channelJoined"`
	PolicyAcceptedAt  *time.Time `json:"policyAcceptedAt"`
	AgreementRevision int        `json:"agreementRevision"`
	DisplayCurrency   string     `json:"displayCurrency"`
	RecoveryReason    string     `json:"recoveryReason"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type authState struct {
	Authenticated bool                   `json:"authenticated"`
	User          userResponse           `json:"user"`
	Captcha       *accounts.CaptchaState `json:"captcha,omitempty"`
}

func mapUser(user model.User) userResponse {
	displayName := user.DisplayName()
	displayCurrency, ok := currencydisplay.ParseCurrency(user.DisplayCurrency)
	if !ok {
		displayCurrency = currencydisplay.TXB
	}
	return userResponse{
		ID: user.ID, TelegramID: strconv.FormatInt(user.TelegramID, 10), FirstName: user.TelegramFirstName,
		LastName: user.TelegramLastName, TelegramUsername: user.TelegramUsername, DisplayName: displayName,
		Username: &displayName, Role: user.Role,
		OnboardingState: user.OnboardingState, GroupJoined: user.GroupJoined, ChannelJoined: user.ChannelJoined,
		PolicyAcceptedAt: user.PolicyAcceptedAt, AgreementRevision: user.AgreementRevision, RecoveryReason: user.RecoveryReason,
		DisplayCurrency: string(displayCurrency),
		CreatedAt:       user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		InitData string `json:"initData"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "initData is required.")
		return
	}
	user, token, expiresAt, err := s.deps.Accounts.Authenticate(r.Context(), request.InitData)
	if err != nil {
		if errors.Is(err, accounts.ErrAuthenticationRateLimited) {
			w.Header().Set("Retry-After", "30")
			s.writeError(w, r, http.StatusTooManyRequests, "AUTH_RATE_LIMITED", "Too many authentication attempts. Please retry shortly.")
			return
		}
		if errors.Is(err, accounts.ErrUpstreamUnavailable) {
			s.writeError(w, r, http.StatusServiceUnavailable, "REMNAWAVE_UNAVAILABLE", "Account verification is temporarily unavailable. Please retry.")
			return
		}
		s.deps.Logger.Warn("Telegram Mini App authentication rejected", "request_id", middlewareRequestID(r), "error", err)
		s.writeError(w, r, http.StatusUnauthorized, "INVALID_TELEGRAM_DATA", "Telegram authentication could not be verified.")
		return
	}
	clientKey, err := s.requests.ClientKey(token)
	if err != nil {
		s.deps.Logger.Error("derive browser request key", "request_id", middlewareRequestID(r), "error", err)
		s.writeError(w, r, http.StatusInternalServerError, "SESSION_CREATE_FAILED", "The secure session could not be created.")
		return
	}
	state, err := s.panelAuthState(r, user, true)
	if err != nil {
		s.captchaFailure(w, r, err)
		return
	}
	s.setSessionCookies(w, token, clientKey, expiresAt)
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) setSessionCookies(w http.ResponseWriter, token, clientKey string, expiresAt time.Time) {
	common := http.Cookie{Path: "/", Expires: expiresAt, MaxAge: int(s.deps.SessionTTL.Seconds()),
		Secure: s.deps.SecureCookies, SameSite: http.SameSiteLaxMode}
	session := common
	session.Name, session.Value, session.HttpOnly = sessionCookie, token, true
	http.SetCookie(w, &session)
	requestKey := common
	requestKey.Name, requestKey.Value = requestauth.ClientKeyCookie, clientKey
	http.SetCookie(w, &requestKey)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	state, err := s.panelAuthState(r, currentUser(r), true)
	if err != nil {
		s.captchaFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}
