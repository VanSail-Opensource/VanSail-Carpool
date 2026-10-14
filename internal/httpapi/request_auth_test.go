package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
)

const httpTestSession = "c2Vzc2lvbi10b2tlbi0zMi1ieXRlcy0wMDAwMDAwMDA"

func TestRequireSignedRequest(t *testing.T) {
	t.Parallel()
	verifier, err := requestauth.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("requestauth.New(): %v", err)
	}
	server := &Server{requests: verifier}
	called := false
	handler := server.requireSignedRequest(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/purchases?mode=now", strings.NewReader(`{"comboId":"combo-1"}`))
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: httpTestSession})
	clientKey, err := verifier.ClientKey(httpTestSession)
	if err != nil {
		t.Fatalf("ClientKey(): %v", err)
	}
	request.AddCookie(&http.Cookie{Name: requestauth.ClientKeyCookie, Value: clientKey})
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "AAAAAAAAAAAAAAAAAAAAAA"
	signature, err := requestauth.Sign(clientKey, request.Method, requestauth.CanonicalTarget(request.URL), timestamp, nonce, []byte(`{"comboId":"combo-1"}`))
	if err != nil {
		t.Fatalf("Sign(): %v", err)
	}
	request.Header.Set(requestauth.TimestampHeader, timestamp)
	request.Header.Set(requestauth.NonceHeader, nonce)
	request.Header.Set(requestauth.SignatureHeader, signature)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("signed response = %d, called = %t", response.Code, called)
	}
}

func TestRequireSignedRequestRejectsMissingCompanionCookie(t *testing.T) {
	t.Parallel()
	verifier, err := requestauth.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("requestauth.New(): %v", err)
	}
	server := &Server{requests: verifier}
	handler := server.requireSignedRequest(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unsigned request reached handler")
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: httpTestSession})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestSetSessionCookiesKeepsOnlySessionHTTPOnly(t *testing.T) {
	t.Parallel()
	server := &Server{deps: Dependencies{SessionTTL: time.Hour, SecureCookies: true}}
	response := httptest.NewRecorder()
	server.setSessionCookies(response, "session", "client-key", time.Now().Add(time.Hour))
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookie count = %d, want 2", len(cookies))
	}
	byName := map[string]*http.Cookie{cookies[0].Name: cookies[0], cookies[1].Name: cookies[1]}
	if !byName[sessionCookie].HttpOnly || byName[requestauth.ClientKeyCookie].HttpOnly {
		t.Fatal("session must be HttpOnly and companion key must be browser-readable")
	}
	if !byName[sessionCookie].Secure || !byName[requestauth.ClientKeyCookie].Secure {
		t.Fatal("both cookies must inherit secure transport policy")
	}
}

func TestRequestAuthenticationErrorStatusMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "signed body overflow", err: requestauth.ErrBodyTooLarge, status: http.StatusRequestEntityTooLarge},
		{name: "duplicate nonce", err: fmt.Errorf("%w: duplicate nonce", requestauth.ErrReplay), status: http.StatusConflict},
		{name: "session nonce capacity", err: fmt.Errorf("%w: nonce capacity", requestauth.ErrRateLimited), status: http.StatusTooManyRequests},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := &Server{}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/example", nil)
			server.writeRequestAuthError(response, request, test.err)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
