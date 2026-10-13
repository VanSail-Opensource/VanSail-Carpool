package browserauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

type oauthTestTransport struct {
	run func(*http.Request) (*http.Response, error)
}

func (t oauthTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t.run(r) }

func signTestToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test"}`))
	signing := header + "." + base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(signing))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func TestTelegramOIDCVerifiesClaimsAndNumericIDIndependentlyOfSubject(t *testing.T) {
	service, user := testService(t)
	ctx := context.Background()
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	if _, err := service.SaveConfig(ctx, user.ID, Config{Enabled: true, ClientID: "12345", ClientSecret: "secret"}); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	public := fmt.Sprintf(`{"keys":[{"kty":"RSA","use":"sig","alg":"RS256","kid":"test","n":"%s","e":"%s"}]}`, base64.RawURLEncoding.EncodeToString(key.N.Bytes()), base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()))
	for _, test := range []struct {
		name  string
		patch func(map[string]any)
		valid bool
	}{
		{"verified numeric ID", func(map[string]any) {}, true},
		{"missing profile ID", func(c map[string]any) { delete(c, "id") }, false},
		{"missing issued time", func(c map[string]any) { delete(c, "iat") }, false},
		{"missing subject", func(c map[string]any) { delete(c, "sub") }, false},
		{"empty subject", func(c map[string]any) { c["sub"] = "" }, false},
		{"string profile ID", func(c map[string]any) { c["id"] = "80304" }, false},
		{"invalid issuer", func(c map[string]any) { c["iss"] = "https://evil.example" }, false},
		{"invalid audience", func(c map[string]any) { c["aud"] = "other-bot" }, false},
		{"expired", func(c map[string]any) { c["exp"] = now.Add(-time.Hour).Unix() }, false},
		{"invalid nonce", func(c map[string]any) { c["nonce"] = "wrong" }, false},
		{"future issue", func(c map[string]any) { c["iat"] = now.Add(10 * time.Minute).Unix() }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			flow := OAuthFlow{State: "state", Verifier: "verifier", Nonce: "nonce", ClientID: "12345", ReturnTo: "/admin/settings", Expires: now.Add(5 * time.Minute)}
			claims := map[string]any{"iss": telegramIssuer, "aud": "12345", "sub": "a-different-oidc-subject", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "id": user.TelegramID, "nonce": flow.Nonce, "given_name": "Verified"}
			test.patch(claims)
			encoded := signTestToken(t, key, claims)
			tokenCalls, keyCalls := 0, 0
			client := &http.Client{Transport: oauthTestTransport{run: func(r *http.Request) (*http.Response, error) {
				var body string
				switch r.URL.Path {
				case "/token":
					tokenCalls++
					if err := r.ParseForm(); err != nil {
						return nil, err
					}
					id, secret, ok := r.BasicAuth()
					if !ok || id != "12345" || secret != "secret" || r.Form.Get("code_verifier") != flow.Verifier || r.Form.Get("redirect_uri") != service.CallbackURL() || r.Form.Get("code") != "code" {
						return nil, fmt.Errorf("incorrect token exchange contract")
					}
					body = `{"id_token":"` + encoded + `"}`
				case "/.well-known/jwks.json":
					keyCalls++
					body = public
				default:
					return nil, fmt.Errorf("unexpected endpoint %s", r.URL.Path)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			}}}
			current := New(service.store, service.vault, client, service.publicURL, []int64{user.TelegramID})
			current.now = service.now
			profile, err := current.CompleteOAuth(ctx, flow, flow.State, "code")
			if (err == nil) != test.valid {
				t.Fatalf("profile=%+v,error=%v,valid=%v", profile, err, test.valid)
			}
			if test.valid && (profile.ID != user.TelegramID || profile.FirstName != "Verified" || tokenCalls != 1 || keyCalls != 1) {
				t.Fatalf("profile=%+v, token=%d,keys=%d", profile, tokenCalls, keyCalls)
			}
			tokenCalls = 0
			if _, err = current.CompleteOAuth(ctx, flow, "wrong-state", "code"); err == nil || tokenCalls != 0 {
				t.Fatal("unbound state contacted Telegram")
			}
		})
	}
}
