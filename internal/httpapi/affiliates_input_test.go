package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
)

type affiliateInputStore struct {
	affiliates.Repository
	writes int
}

func (store *affiliateInputStore) SaveAffiliateConfig(_ context.Context, _ string, input affiliates.ConfigInput, _ time.Time) (affiliates.Config, error) {
	store.writes++
	return affiliates.Config{Version: input.ExpectedVersion + 1, Tiers: input.Tiers}, nil
}

func TestAffiliateSettingsRejectMalformedDocumentsBeforeMutation(t *testing.T) {
	t.Parallel()
	const valid = `{"expectedVersion":1,"tiers":[{"name":"Base","threshold":0,"enabled":true,"reward":{"kind":"none"}}]}`
	for _, test := range []struct {
		name string
		body string
	}{
		{"second document", valid + `{}`},
		{"trailing garbage", valid + `!`},
		{"invalid UTF-8", strings.Replace(valid, "Base", string([]byte{'B', 0xff}), 1)},
		{"control character", strings.Replace(valid, "Base", `Base\u0000`, 1)},
		{"oversized trailing whitespace", valid + strings.Repeat(" ", 64<<10)},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &affiliateInputStore{}
			server := &Server{deps: Dependencies{Affiliates: affiliates.NewService(store, nil)}}
			response := httptest.NewRecorder()
			server.adminUpdateAffiliates(response, httptest.NewRequest(http.MethodPut, "/api/v1/admin/affiliates", strings.NewReader(test.body)))
			if response.Code != http.StatusBadRequest || store.writes != 0 {
				t.Fatalf("malformed document: status %d, writes %d", response.Code, store.writes)
			}
		})
	}
	store := &affiliateInputStore{}
	server := &Server{deps: Dependencies{Affiliates: affiliates.NewService(store, nil)}}
	response := httptest.NewRecorder()
	server.adminUpdateAffiliates(response, httptest.NewRequest(http.MethodPut, "/api/v1/admin/affiliates", strings.NewReader(valid)))
	if response.Code != http.StatusOK || store.writes != 1 {
		t.Fatalf("valid document: status %d, writes %d", response.Code, store.writes)
	}
}
