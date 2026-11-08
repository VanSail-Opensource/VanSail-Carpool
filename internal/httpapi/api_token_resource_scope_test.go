package httpapi

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAPITokenNativeRoutesAndReceiptRequireExplicitResource(t *testing.T) {
	s, handler, user := testBrowserHTTP(t)
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{user.TelegramID})
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession, s.requireAdmin)
		r.Get("/api/v1/admin/xnode/service-identities/{id}/subscription", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
		r.Post("/api/v1/admin/xnode/nodes/{id}/enrollment", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
		r.Get("/api/v1/admin/operations/{operationId}", s.adminOperationReceipt)
	})
	ctx := context.Background()
	op, _, err := s.deps.Store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: user.ID, Kind: "xnode_command", IdempotencyKey: "native-scope", RequestFingerprint: strings.Repeat("f", 64), SealedTarget: "private"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		scopes      []string
		read, write int
	}{
		{[]string{"admin:read"}, 403, 403},
		{[]string{"admin:write"}, 403, 403},
		{[]string{"xnode:read"}, 204, 403},
		{[]string{"xnode:write"}, 204, 204},
		{[]string{"admin:write", "xnode:read"}, 204, 403},
	} {
		issued, err := s.deps.APITokens.Create(ctx, user, apitoken.CreateInput{Name: "resource scope", Scopes: test.scopes})
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range []struct {
			method, path string
			want         int
		}{
			{"GET", "/api/v1/admin/xnode/service-identities/identity/subscription", test.read},
			{"POST", "/api/v1/admin/xnode/nodes/node/enrollment", test.write},
			{"GET", "/api/v1/admin/operations/" + op.Receipt.ID, map[bool]int{true: 200, false: 403}[test.read == 204]},
		} {
			request := httptest.NewRequest(entry.method, entry.path, nil)
			request.Header.Set("Authorization", "Bearer "+issued.Token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != entry.want {
				t.Fatalf("scopes %v %s %s: %d want %d", test.scopes, entry.method, entry.path, response.Code, entry.want)
			}
		}
	}
}
