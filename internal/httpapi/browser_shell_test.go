package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBrowserConsoleCannotBeEmbeddedAndDoesNotPermitTelegramSDK(t *testing.T) {
	server := &Server{}
	handler := server.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for _, test := range []struct {
		path  string
		admin bool
	}{{"/admin", true}, {"/admin/settings", true}, {"/admin/users/id", true}, {"/admin.html", true}, {"/settings", false}, {"/", false}} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			policy := response.Header().Get("Content-Security-Policy")
			if test.admin {
				if !strings.Contains(policy, "frame-ancestors 'none'") || strings.Contains(policy, "telegram.org") || strings.Contains(policy, "cloudflare.com") {
					t.Fatalf("admin policy=%q", policy)
				}
			} else if !strings.Contains(policy, "frame-ancestors https://web.telegram.org https://*.telegram.org") {
				t.Fatalf("Mini App cannot be embedded: %q", policy)
			}
		})
	}
}

func TestMissingBrowserEntryNeverFallsBackToMiniApp(t *testing.T) {
	handler := spaHandler(fstest.MapFS{"index.html": {Data: []byte("member only")}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/users/id", nil))
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "member only") {
		t.Fatalf("missing browser entry served member shell=%d:%s", response.Code, response.Body.String())
	}
}
