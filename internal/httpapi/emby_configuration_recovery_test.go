package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type embyConfigurationSettings struct {
	admin.SettingsRepository
	record database.EmbyConnectionRecord
	err    error
}

func (r *embyConfigurationSettings) EmbyConnection(context.Context) (database.EmbyConnectionRecord, error) {
	return r.record, r.err
}

func (*embyConfigurationSettings) SaveEmbyConnection(context.Context, string, database.EmbyConnectionRecord, string) error {
	return errors.New("unexpected configuration write")
}

type embyConfigurationAccount struct{ emby.Repository }

func (embyConfigurationAccount) EmbyAccountForUser(_ context.Context, userID string) (emby.Account, error) {
	return emby.Account{ID: "account", UserID: userID, Status: emby.StatusActive}, nil
}

type embyConfigurationRemote struct {
	emby.Remote
	calls int
}

func (r *embyConfigurationRemote) ListSelectableFolders(context.Context) ([]emby.Folder, error) {
	r.calls++
	return []emby.Folder{{ID: "library", Name: "Library"}}, nil
}

func (*embyConfigurationRemote) ListParentalRatings(context.Context) ([]emby.ParentalRating, error) {
	return []emby.ParentalRating{}, nil
}

func embyConfigurationRequest(s *Server) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/emby/account", nil)
	request = request.WithContext(context.WithValue(request.Context(), userContextKey, model.User{ID: "member"}))
	response := httptest.NewRecorder()
	s.embyAccount(response, request)
	return response
}

func TestEmbyConfigurationStorageOutageDoesNotPublishDisabledState(t *testing.T) {
	for _, source := range []string{"connection", "price"} {
		t.Run(source, func(t *testing.T) {
			settings := &embyConfigurationSettings{record: database.EmbyConnectionRecord{
				BaseURL: database.Setting{Value: "https://emby.example"}, Token: database.Setting{Value: "token"},
			}}
			var priceErr error
			prices := emby.PriceFunc(func(context.Context) (int64, error) { return 1250, priceErr })
			remote := &embyConfigurationRemote{}
			s := &Server{deps: Dependencies{Settings: admin.NewSettingsService(settings, nil), EmbyPrice: prices,
				Emby: emby.NewService(embyConfigurationAccount{}, remote, prices, nil)}}
			for _, unavailable := range []bool{false, true, false} {
				var outage error
				if unavailable {
					outage = errors.New("private SQL connection error with secret credential")
				}
				if source == "connection" {
					settings.err = outage
				} else {
					priceErr = outage
				}
				response := embyConfigurationRequest(s)
				if unavailable {
					var failure apiError
					if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
						t.Fatal(err)
					}
					if response.Code != http.StatusServiceUnavailable || failure.Code != "EMBY_UNAVAILABLE" || strings.Contains(response.Body.String(), "configured") || strings.Contains(response.Body.String(), "private") || strings.Contains(response.Body.String(), "secret") {
						t.Fatalf("outage published disabled state or private error: %d %s", response.Code, response.Body.String())
					}
					continue
				}
				var state struct {
					Configured bool                  `json:"configured"`
					SetupPrice model.Money           `json:"setupPrice"`
					Account    embyAccountResponse   `json:"account"`
					Libraries  []embyLibraryResponse `json:"libraries"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusOK || !state.Configured || state.SetupPrice != model.TXBMoney(1250) || state.Account.Status != emby.StatusActive || len(state.Libraries) != 1 {
					t.Fatalf("working configuration did not recover: %d %s", response.Code, response.Body.String())
				}
			}
			if remote.calls != 2 {
				t.Fatalf("configuration outage reached upstream: %d option calls", remote.calls)
			}
		})
	}
}

func TestEmbyIncompleteConfigurationRemainsDisabled(t *testing.T) {
	for _, item := range []struct {
		name       string
		connection bool
		priceErr   error
	}{
		{name: "missing credentials"},
		{name: "missing price", connection: true, priceErr: database.ErrNotFound},
		{name: "invalid price", connection: true, priceErr: emby.ErrInvalidSetup},
	} {
		t.Run(item.name, func(t *testing.T) {
			settings := &embyConfigurationSettings{}
			if item.connection {
				settings.record = database.EmbyConnectionRecord{BaseURL: database.Setting{Value: "https://emby.example"}, Token: database.Setting{Value: "token"}}
			}
			prices := emby.PriceFunc(func(context.Context) (int64, error) { return 0, item.priceErr })
			remote := &embyConfigurationRemote{}
			s := &Server{deps: Dependencies{Settings: admin.NewSettingsService(settings, nil), EmbyPrice: prices,
				Emby: emby.NewService(embyConfigurationAccount{}, remote, prices, nil)}}
			response := embyConfigurationRequest(s)
			var state struct {
				Configured bool        `json:"configured"`
				SetupPrice model.Money `json:"setupPrice"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || state.Configured || state.SetupPrice != model.TXBMoney(0) || remote.calls != 0 {
				t.Fatalf("incomplete configuration changed behavior: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
