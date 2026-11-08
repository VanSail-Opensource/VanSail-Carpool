package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRetiredOutageControlsHaveNoAdminRoutes(t *testing.T) {
	router := chi.NewRouter()
	server := &Server{}
	server.mountAdmin(router)
	for _, target := range []struct{ method, path string }{
		{http.MethodGet, "/node-compensation/config"},
		{http.MethodPut, "/node-compensation/config"},
		{http.MethodGet, "/node-compensation/events"},
		{http.MethodPost, "/node-compensation/events/old/approve"},
		{http.MethodPost, "/node-compensation/events/old/dismiss"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(target.method, target.path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("retired route %s %s returned %d", target.method, target.path, response.Code)
		}
	}
}
