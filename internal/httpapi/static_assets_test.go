package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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
