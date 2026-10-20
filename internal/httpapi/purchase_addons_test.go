package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestUnexpectedSquadAdditionErrorKeepsConflictContract(t *testing.T) {
	s := &Server{deps: Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/purchases/purchase-1/addons", nil)
	response := httptest.NewRecorder()
	s.writePurchaseAddonError(response, request, fmt.Errorf("commit purchase add-on: database unavailable"))
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusConflict || body.Code != "SQUAD_ADDITION_FAILED" {
		t.Fatalf("unexpected add-on error = %d %s", response.Code, response.Body.String())
	}
}

func TestSquadAdditionOverrideConflict(t *testing.T) {
	s := &Server{}
	response := httptest.NewRecorder()
	s.writePurchaseAddonError(response, httptest.NewRequest(http.MethodPost, "/addons", nil), fmt.Errorf("quote: %w", database.ErrAddonEntitlementOverride))
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusConflict || body.Code != "SQUAD_ADDITION_OVERRIDDEN" {
		t.Fatal(response.Code, response.Body.String())
	}
}
