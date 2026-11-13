package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBrowserExchangeTransientFailureRetainsUsableHandoff(t *testing.T) {
	server, handler, user := testBrowserHTTP(t)
	link, _, err := server.deps.BrowserAuth.Link(context.Background(), user, "/admin/settings")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{"token": fragment.Get("token")})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		name   string
		ctx    context.Context
		status int
		code   string
	}{
		{"transient request cancellation", cancelled, http.StatusServiceUnavailable, "BROWSER_SESSION_UNAVAILABLE"},
		{"same link succeeds after retry", context.Background(), http.StatusOK, ""},
		{"consumed link remains one use", context.Background(), http.StatusUnauthorized, "BROWSER_LINK_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/browser/exchange", strings.NewReader(string(body))).WithContext(test.ctx)
			request.Header.Set("Origin", "https://panel.example")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || test.code != "" && !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("exchange response = %d: %s", response.Code, response.Body.String())
			}
			if test.status != http.StatusOK && len(response.Result().Cookies()) != 0 {
				t.Fatal("failed handoff issued session cookies")
			}
			if test.status == http.StatusOK && len(response.Result().Cookies()) != 2 {
				t.Fatal("successful retry did not issue the browser cookie pair")
			}
		})
	}
}
