package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
)

type namedServiceErrorRepository struct {
	xnode.Repository
	identity xnode.Identity
	writes   int
}

func (r *namedServiceErrorRepository) XNodeIdentity(context.Context, int64) (xnode.Identity, error) {
	return r.identity, nil
}

func (r *namedServiceErrorRepository) SaveXNodeIdentity(_ context.Context, identity xnode.Identity) (xnode.Identity, error) {
	r.writes++
	return identity, nil
}

func TestNamedServiceRotationPreservesCredentialReadErrors(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	probeCipher, err := vault.Encrypt("xnode.identity.service.test", "{}")
	if err != nil {
		t.Fatal(err)
	}
	invalidTermCipher, err := vault.Encrypt("xnode.identity.service.test", `{"serviceAccess":{"limitBytes":"-1"}}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		cipher string
		status int
		code   string
	}{
		{name: "unreadable credentials", cipher: "v1:invalid", status: http.StatusServiceUnavailable, code: "XNODE_UNAVAILABLE"},
		{name: "valid probe", cipher: probeCipher, status: http.StatusNotFound, code: "XNODE_NOT_FOUND"},
		{name: "invalid stored term", cipher: invalidTermCipher, status: http.StatusUnprocessableEntity, code: "XNODE_INPUT_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &namedServiceErrorRepository{identity: xnode.Identity{
				ProxyUserID: 1<<52 + 1, ServiceName: "test", CredentialsCipher: test.cipher,
			}}
			native := xnode.NewService(repository, nil, vault, &url.URL{Scheme: "https", Host: "panel.example"}, nil)
			server := &Server{deps: Dependencies{XNode: native}}
			router := chi.NewRouter()
			router.Post("/services/{id}/rotate", server.xnodeRotateServiceSubscription)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/services/"+strconv.FormatInt(repository.identity.ProxyUserID, 10)+"/rotate", nil))
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if repository.writes != 0 || strings.Contains(response.Body.String(), test.cipher) {
				t.Fatalf("failed read rotated or exposed credentials: writes=%d body=%s", repository.writes, response.Body.String())
			}
		})
	}
}
