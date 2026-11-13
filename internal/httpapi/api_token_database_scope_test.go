package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/go-chi/chi/v5"
)

func TestAPITokenDatabaseAndBackupsRequireBothDomains(t *testing.T) {
	s, handler, user := testBrowserHTTP(t)
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{user.TelegramID})
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession, s.requireAdmin)
		accepted := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
		r.Get("/api/v1/admin/database/tables/{table}/rows", accepted)
		r.Post("/api/v1/admin/database/tables/{table}/query", accepted)
		r.Post("/api/v1/admin/database/mutations/review", accepted)
		r.Post("/api/v1/admin/database/mutations", accepted)
		r.Put("/api/v1/admin/database/tables/{table}/rows", accepted)
		r.Get("/api/v1/admin/backups/{id}/download", accepted)
	})
	for _, test := range []struct {
		scopes      []string
		read, write int
	}{
		{[]string{"admin:read"}, 403, 403},
		{[]string{"admin:write"}, 403, 403},
		{[]string{"xnode:write"}, 403, 403},
		{[]string{"admin:read", "xnode:read"}, 204, 403},
		{[]string{"admin:write", "xnode:read"}, 204, 403},
		{[]string{"admin:read", "xnode:write"}, 204, 403},
		{[]string{"admin:write", "xnode:write"}, 204, 204},
	} {
		issued, err := s.deps.APITokens.Create(context.Background(), user, apitoken.CreateInput{Name: "database scope", Scopes: test.scopes})
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range []struct {
			method, path string
			want         int
		}{
			{"GET", "/api/v1/admin/database/tables/xnode_nodes/rows", test.read},
			{"POST", "/api/v1/admin/database/tables/xnode_nodes/query", test.read},
			{"POST", "/api/v1/admin/database/mutations/review", test.write},
			{"POST", "/api/v1/admin/database/mutations", test.write},
			{"PUT", "/api/v1/admin/database/tables/xnode_nodes/rows", test.write},
			{"GET", "/api/v1/admin/backups/backup/download", test.read},
		} {
			request := httptest.NewRequest(entry.method, entry.path, nil)
			request.Header.Set("Authorization", "Bearer "+issued.Token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != entry.want {
				t.Errorf("scopes %v %s %s: status %d want %d", test.scopes, entry.method, entry.path, response.Code, entry.want)
			}
		}
	}
}
