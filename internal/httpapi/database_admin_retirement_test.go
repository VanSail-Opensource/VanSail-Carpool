package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDatabaseAdministrationRejectsRetiredRowUpdateRoute(t *testing.T) {
	router := chi.NewRouter()
	NewDatabaseAdministrationHTTP(nil, nil).Mount(router)
	request := httptest.NewRequest(http.MethodPut, "/database/tables/users/rows", strings.NewReader(`{"values":{"role":"admin"}}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatal("retired database mutation route remained reachable", response.Code)
	}
}
