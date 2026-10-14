package httpapi

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreferenceTransportOwnsPrincipalAndRejectsClientIdentity(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "prefs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store := database.NewStore(db)
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 55701, FirstName: "Lin"}, false)
	if err != nil {
		t.Fatal(err)
	}
	user.OnboardingState = "complete"
	server := &Server{deps: Dependencies{Store: store}}
	request := communityRequest(http.MethodPatch, "/api/v1/me/preferences", user)
	request.Body = http.NoBody
	request = httptest.NewRequest(http.MethodPatch, "/api/v1/me/preferences", strings.NewReader(`{"notifications":{"money":false},"showActivity":true}`)).WithContext(request.Context())
	response := httptest.NewRecorder()
	server.updateUserPreferences(response, request)
	if response.Code != 200 {
		t.Fatalf("update=%d %s", response.Code, response.Body.String())
	}
	var value preferences.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil || value.Notifications.Money || value.ShowActivity {
		t.Fatalf("response=%+v %v", value, err)
	}
	request = httptest.NewRequest(http.MethodPatch, "/api/v1/me/preferences", strings.NewReader(`{"userId":"other","includeNodePrices":false}`)).WithContext(request.Context())
	response = httptest.NewRecorder()
	server.updateUserPreferences(response, request)
	if response.Code != 400 {
		t.Fatalf("client identity=%d", response.Code)
	}
	unboarded := user
	unboarded.OnboardingState = "username"
	response = httptest.NewRecorder()
	server.userPreferences(response, communityRequest(http.MethodGet, "/api/v1/me/preferences", unboarded))
	if response.Code != http.StatusForbidden && response.Code != http.StatusConflict {
		t.Fatalf("onboarding gate=%d", response.Code)
	}
	stored, err := store.UserPreferenceSnapshot(ctx, user.ID, time.Now().UTC())
	if err != nil || !stored.IncludeNodePrices {
		t.Fatalf("rejected input mutated preferences: %+v %v", stored, err)
	}
}
