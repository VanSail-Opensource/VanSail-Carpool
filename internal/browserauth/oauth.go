package browserauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
)

const telegramIssuer = "https://oauth.telegram.org"

// OAuthFlow holds a browser-bound state, PKCE verifier, nonce, and fixed expiry inside an encrypted cookie.
type OAuthFlow struct {
	State    string    `json:"state"`
	Verifier string    `json:"verifier"`
	Nonce    string    `json:"nonce"`
	ClientID string    `json:"clientId"`
	ReturnTo string    `json:"returnTo"`
	Expires  time.Time `json:"expires"`
}

// StartOAuth constructs a five-minute authorization flow without requesting phone or bot-access permission.
func (s *Service) StartOAuth(ctx context.Context, returnTo string) (OAuthFlow, string, error) {
	config, err := s.Config(ctx)
	if err != nil {
		return OAuthFlow{}, "", err
	}
	if !config.Enabled || config.ClientID == "" || config.ClientSecret == "" || s.client == nil {
		return OAuthFlow{}, "", errors.New("Telegram OAuth is unavailable")
	}
	if returnTo == "" {
		returnTo = "/admin/settings"
	}
	if !ValidDestination(returnTo) {
		return OAuthFlow{}, "", errors.New("invalid destination")
	}
	state, err := ids.Token(32)
	if err != nil {
		return OAuthFlow{}, "", err
	}
	verifier, err := ids.Token(32)
	if err != nil {
		return OAuthFlow{}, "", err
	}
	nonce, err := ids.Token(32)
	if err != nil {
		return OAuthFlow{}, "", err
	}
	flow := OAuthFlow{State: state, Verifier: verifier, Nonce: nonce, ClientID: config.ClientID, ReturnTo: returnTo, Expires: s.now().UTC().Add(5 * time.Minute)}
	challenge := sha256.Sum256([]byte(verifier))
	params := url.Values{"client_id": {config.ClientID}, "redirect_uri": {s.CallbackURL()}, "response_type": {"code"}, "scope": {"openid profile"}, "state": {state}, "nonce": {nonce}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}}
	return flow, telegramIssuer + "/auth?" + params.Encode(), nil
}

// CompleteOAuth exchanges the single-use code through the injected queued client and verifies Telegram's ID token.
func (s *Service) CompleteOAuth(ctx context.Context, flow OAuthFlow, state, code string) (model.TelegramProfile, error) {
	if !flow.Expires.After(s.now().UTC()) || len(code) == 0 || len(code) > 4096 || subtle.ConstantTimeCompare([]byte(state), []byte(flow.State)) != 1 {
		return model.TelegramProfile{}, ErrUnauthorized
	}
	config, err := s.Config(ctx)
	if err != nil {
		return model.TelegramProfile{}, err
	}
	if !config.Enabled || config.ClientID != flow.ClientID || s.client == nil {
		return model.TelegramProfile{}, ErrUnauthorized
	}
	params := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {s.CallbackURL()}, "client_id": {config.ClientID}, "code_verifier": {flow.Verifier}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramIssuer+"/token", strings.NewReader(params.Encode()))
	if err != nil {
		return model.TelegramProfile{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(config.ClientID, config.ClientSecret)
	response, err := s.client.Do(request)
	if err != nil {
		return model.TelegramProfile{}, errors.New("Telegram token exchange unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return model.TelegramProfile{}, ErrUnauthorized
	}
	var token struct {
		IDToken string `json:"id_token"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token); err != nil || token.IDToken == "" {
		return model.TelegramProfile{}, ErrUnauthorized
	}
	verifier := oidc.NewVerifier(telegramIssuer, s.keys, &oidc.Config{ClientID: config.ClientID, SupportedSigningAlgs: []string{oidc.RS256, oidc.ES256}, Now: s.now})
	verified, err := verifier.Verify(ctx, token.IDToken)
	if err != nil || verified.IssuedAt.IsZero() || verified.Subject == "" || subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(flow.Nonce)) != 1 || verified.IssuedAt.After(s.now().Add(time.Minute)) {
		return model.TelegramProfile{}, ErrUnauthorized
	}
	return telegramProfile(verified)
}

func telegramProfile(token *oidc.IDToken) (model.TelegramProfile, error) {
	var claims struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		FirstName string `json:"given_name"`
		LastName  string `json:"family_name"`
		Username  string `json:"preferred_username"`
	}
	if err := token.Claims(&claims); err != nil || claims.ID <= 0 {
		return model.TelegramProfile{}, ErrUnauthorized
	}
	if claims.FirstName == "" {
		claims.FirstName = claims.Name
	}
	return model.TelegramProfile{ID: claims.ID, FirstName: claims.FirstName, LastName: claims.LastName, Username: claims.Username}, nil
}

// SealCookie encrypts short-lived, browser-bound OAuth state without storing it in the database.
func (s *Service) SealCookie(name string, value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return s.vault.Encrypt(name, string(raw))
}

// OpenCookie authenticates and decodes browser-bound state. Callers enforce the embedded expiry.
func (s *Service) OpenCookie(name, encoded string, target any) error {
	if len(encoded) > 8192 {
		return ErrUnauthorized
	}
	raw, err := s.vault.Decrypt(name, encoded)
	if err != nil {
		return ErrUnauthorized
	}
	return json.Unmarshal([]byte(raw), target)
}
