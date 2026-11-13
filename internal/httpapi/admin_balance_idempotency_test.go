package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/go-chi/chi/v5"
)

func TestAdminBalanceAdjustmentBindsRetryToExactMutation(t *testing.T) {
	s, handler, actor := testBrowserHTTP(t)
	s.deps.Admin = admin.NewService(s.deps.Store, nil, nil, nil, nil)
	s.deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{actor.TelegramID})
	issued, err := s.deps.APITokens.Create(context.Background(), actor, apitoken.CreateInput{Name: "balance retry", Scopes: []string{"admin:write"}})
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := s.deps.Store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: actor.TelegramID + 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession, s.requireAdmin)
		r.Post("/api/v1/admin/users/{id}/balance-adjustments", s.adminBalanceAdjustment)
	})
	var firstID string
	for _, test := range []struct {
		name, target, key, amount, reason string
		status                            int
	}{
		{"missing key", actor.ID, "", "250", "correction", 400},
		{"first attempt", actor.ID, "stable", "250", "correction", 201},
		{"retry after response loss", actor.ID, "stable", "250", "correction", 201},
		{"different amount", actor.ID, "stable", "300", "correction", 409},
		{"different reason", actor.ID, "stable", "250", "another correction", 409},
		{"different target", other.ID, "stable", "250", "correction", 409},
		{"new adjustment", actor.ID, "next", "100", "correction", 201},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"deltaTxbMinor": test.amount, "reason": test.reason})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+test.target+"/balance-adjustments", strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer "+issued.Token)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", test.key)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status %d want %d: %s", response.Code, test.status, response.Body.String())
			}
			if test.status == 201 && test.key == "stable" {
				var entry model.LedgerEntry
				if err := json.Unmarshal(response.Body.Bytes(), &entry); err != nil {
					t.Fatal(err)
				}
				if firstID != "" && firstID != entry.ID {
					t.Fatal("retry created a second ledger entry")
				}
				firstID = entry.ID
			}
		})
	}
	for userID, want := range map[string]string{actor.ID: "350", other.ID: "0"} {
		balance, err := s.deps.Store.Balance(context.Background(), userID)
		if err != nil || balance.Minor != want {
			t.Fatalf("balance for %s = %s, %v; want %s", userID, balance.Minor, err, want)
		}
	}
}
