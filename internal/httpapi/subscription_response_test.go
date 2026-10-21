package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
)

func TestEmptySubscriptionResponsesHaveZeroBodyBytes(t *testing.T) {
	for _, status := range []int{200, 204, 304, 403, 599} {
		r := httptest.NewRequest(http.MethodGet, "/subscription/token", nil)
		w := httptest.NewRecorder()
		if !(&Server{}).subscriptionSpecialResponse(w, r, xnode.SubscriptionMetadata{ResponseKind: "empty-http", StatusCode: status}) || w.Code != status || w.Body.Len() != 0 {
			t.Fatal(status, w.Code, w.Body.String())
		}
	}
}

func TestBrowserSubscriptionActionServesPageAndRedirectsRawDownload(t *testing.T) {
	s := &Server{deps: Dependencies{Static: fstest.MapFS{"subscription.html": &fstest.MapFile{Data: []byte("public subscription page")}}}}
	router := chi.NewRouter()
	for _, route := range []string{"/subscription/{token}", "/api/v1/subscriptions/{token}"} {
		router.Get(route, func(w http.ResponseWriter, r *http.Request) {
			s.subscriptionSpecialResponse(w, r, xnode.SubscriptionMetadata{ResponseKind: "browser", StatusCode: 200})
		})
	}
	for _, path := range []string{"/subscription/bearer", "/api/v1/subscriptions/bearer"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if path == "/subscription/bearer" {
			if w.Code != 200 || w.Body.String() != "public subscription page" {
				t.Fatal(w.Code, w.Body.String())
			}
		} else if w.Code != 303 || w.Header().Get("Location") != "/subscription/bearer" {
			t.Fatal(w.Code, w.Header())
		}
	}
}
