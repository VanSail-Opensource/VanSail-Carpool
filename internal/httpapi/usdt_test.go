package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"github.com/go-chi/chi/v5"
)

func TestUSDTAdminRoutesMaskSecretsAndRequireConfiguredAdministrator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "usdt-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := database.NewStore(db)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	settings := admin.NewSettingsService(store, vault)
	collector := usdt.NewService(store, settings)
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99001, FirstName: "operator"}, true)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{deps: Dependencies{USDT: collector, Settings: settings}, adminTelegramIDs: map[int64]struct{}{user.TelegramID: {}}}
	router := chi.NewRouter()
	server.mountUSDT(router)
	request := func(method, path, body string, actor model.User) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), userContextKey, actor))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	member := user
	member.Role = "member"
	if w := request("GET", "/api/v1/admin/usdt", "", member); w.Code != http.StatusForbidden {
		t.Fatal("member accessed settings", w.Code)
	}
	unknown := user
	unknown.TelegramID++
	if w := request("GET", "/api/v1/admin/usdt", "", unknown); w.Code != http.StatusForbidden {
		t.Fatal("unconfigured administrator accessed settings", w.Code)
	}
	c := usdt.DefaultConfig()
	c.Concurrency = 1
	c.Networks[1].Endpoints = []usdt.Endpoint{{ID: "node", RPCURL: "https://rpc.example/secret-path?api=secret-query", APIKey: "secret-key"}}
	body, _ := json.Marshal(c)
	w := request("PUT", "/api/v1/admin/usdt", string(body), user)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "secret-") {
		t.Fatalf("save=%d %s", w.Code, w.Body.String())
	}
	var masked usdt.Config
	if err := json.Unmarshal(w.Body.Bytes(), &masked); err != nil {
		t.Fatal(err)
	}
	if !masked.Networks[1].Endpoints[0].RPCConfigured {
		t.Fatal("masked credential status missing")
	}
	body, _ = json.Marshal(masked)
	if w := request("PUT", "/api/v1/admin/usdt", string(body), user); w.Code != http.StatusOK {
		t.Fatal("blank secret update failed", w.Body.String())
	}
	stored, err := store.GetSetting(ctx, usdt.ConfigKey)
	if err != nil || !stored.Encrypted || strings.Contains(stored.Value, "secret-") {
		t.Fatal("credential stored in plaintext")
	}
	raw, err := settings.Plaintext(ctx, usdt.ConfigKey)
	if err != nil || !strings.Contains(raw, "secret-key") {
		t.Fatal("blank update erased credential")
	}
	for _, path := range []string{"/api/v1/admin/usdt", "/api/v1/admin/usdt/status", "/api/v1/admin/usdt/review"} {
		if w := request("GET", path, "", user); w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("read %s=%d", path, w.Code)
		}
	}
	if w := request("POST", "/api/v1/admin/usdt/check", `{"network":"invalid"}`, user); w.Code != http.StatusUnprocessableEntity {
		t.Fatal("invalid chain accepted", w.Code)
	}
	if w := request("PUT", "/api/v1/admin/usdt/review/invalid/resolution", `{"orderId":"order","reason":"confirmed"}`, user); w.Code != http.StatusUnprocessableEntity {
		t.Fatal("invalid transfer ID accepted", w.Code)
	}
}
