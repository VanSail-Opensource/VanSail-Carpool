package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
)

func TestMissingHashedAssetsNeverBecomeCacheableSPADocuments(t *testing.T) {
	t.Parallel()
	handler := spaHandler(fstest.MapFS{
		"index.html":                {Data: []byte("<main>member shell</main>")},
		"admin.html":                {Data: []byte("<main>admin shell</main>")},
		"assets/profile-current.js": {Data: []byte("export const profile = true")},
	})
	for _, test := range []struct {
		path, contentType, cache string
		status                   int
	}{
		{"/assets/profile-old.js", "text/plain", "no-store", http.StatusNotFound},
		{"/assets/profile-old.css", "text/plain", "no-store", http.StatusNotFound},
		{"/assets/nested/missing.js?version=old", "text/plain", "no-store", http.StatusNotFound},
		{"/assets/profile-current.js", "text/javascript", "public, max-age=31536000, immutable", http.StatusOK},
		{"/admin/users/member-id?tab=history", "text/html", "no-cache, must-revalidate", http.StatusOK},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != test.status || !strings.Contains(response.Header().Get("Content-Type"), test.contentType) || response.Header().Get("Cache-Control") != test.cache {
				t.Fatalf("asset/route response = %d %v", response.Code, response.Header())
			}
			if test.status == http.StatusNotFound && strings.Contains(response.Body.String(), "shell") {
				t.Fatal("missing asset received an SPA shell")
			}
		})
	}
}

func TestUnknownAPIPathsCannotBecomeSuccessfulSPADocuments(t *testing.T) {
	t.Parallel()
	router := chi.NewRouter()
	router.Get("/api/v1/registered", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.Handle("/*", spaHandler(fstest.MapFS{
		"index.html":             {Data: []byte("<main>member shell</main>")},
		"admin.html":             {Data: []byte("<main>admin shell</main>")},
		"api/v1/accidental.json": {Data: []byte("{}")},
	}))
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodDelete} {
		for _, path := range []string{"/api", "/api/", "/api/v1/subscription/ip-blocks", "/api/v2/missing?refresh=1", "/api/v1/accidental.json"} {
			t.Run(method+" "+path, func(t *testing.T) {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
				if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" ||
					strings.Contains(response.Header().Get("Content-Type"), "text/html") || strings.Contains(response.Body.String(), "shell") {
					t.Fatalf("unknown API response = %d %v %q", response.Code, response.Header(), response.Body.String())
				}
			})
		}
	}
	for _, path := range []string{"/apiary", "/admin/users/member-id"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "shell") {
			t.Fatalf("browser route %s lost its shell: %d %q", path, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/registered", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("registered API route was intercepted: %d", response.Code)
	}
}
