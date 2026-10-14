package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestConnectivityEndpointsRequireConfiguredAdministrator(t *testing.T) {
	handler, _, _, _ := connectivityHTTPFixture(t)
	for _, actor := range []model.User{
		{},
		{ID: "member", Role: "user", TelegramID: 42},
		{ID: "forged-admin", Role: "admin", TelegramID: 42},
	} {
		for _, endpoint := range []struct{ method, path, body string }{
			{http.MethodGet, "/host-connectivity", ""},
			{http.MethodGet, "/host-connectivity/history", ""},
			{http.MethodPost, "/host-connectivity/checks", ""},
			{http.MethodPost, "/host-connectivity/test-user/resolve", `{"username":"monitor"}`},
		} {
			response := connectivityHTTPRequest(handler, endpoint.method, "/api/v1/admin"+endpoint.path, endpoint.body, actor)
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "ADMIN_REQUIRED") {
				t.Fatalf("actor=%+v endpoint=%s admitted: status=%d body=%s", actor, endpoint.path, response.Code, response.Body)
			}
		}
	}
}

func TestConnectivityAdminSnapshotResolveAndAsyncDeduplication(t *testing.T) {
	handler, store, probe, host := connectivityHTTPFixture(t)
	actor := model.User{ID: "admin", Role: "admin", TelegramID: 99}
	resolve := connectivityHTTPRequest(handler, http.MethodPost, "/api/v1/admin/host-connectivity/test-user/resolve", `{"username":"monitor"}`, actor)
	var user connectivity.User
	if resolve.Code != http.StatusOK || json.Unmarshal(resolve.Body.Bytes(), &user) != nil || user.ID != 42 {
		t.Fatalf("resolve response: %d %s", resolve.Code, resolve.Body)
	}
	first := connectivityHTTPRequest(handler, http.MethodPost, "/api/v1/admin/host-connectivity/checks", "", actor)
	var firstRun connectivity.Run
	if first.Code != http.StatusAccepted || json.Unmarshal(first.Body.Bytes(), &firstRun) != nil || firstRun.ID == "" {
		t.Fatalf("manual admission response: %d %s", first.Code, first.Body)
	}
	select {
	case <-probe.started:
	case <-time.After(5 * time.Second):
		t.Fatal("accepted check did not reach queued probe")
	}
	duplicate := connectivityHTTPRequest(handler, http.MethodPost, "/api/v1/admin/host-connectivity/checks", "", actor)
	var duplicateRun connectivity.Run
	if duplicate.Code != http.StatusAccepted || json.Unmarshal(duplicate.Body.Bytes(), &duplicateRun) != nil || duplicateRun.ID != firstRun.ID {
		t.Fatalf("duplicate check created another batch: %d %s", duplicate.Code, duplicate.Body)
	}
	close(probe.release)
	connectivityHTTPWait(t, func() bool {
		page, err := store.ConnectivityHistory(context.Background(), host, "", 50, time.Now().UTC())
		return err == nil && len(page.Items) == 1 && page.Items[0].Status == "connected"
	})
	response := connectivityHTTPRequest(handler, http.MethodGet, "/api/v1/admin/host-connectivity", "", actor)
	var snapshot connectivity.Snapshot
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &snapshot) != nil || len(snapshot.Hosts) != 1 ||
		snapshot.Hosts[0].Latest == nil || snapshot.Hosts[0].Latest.RunID != firstRun.ID || strings.Contains(response.Body.String(), "private-fixture-secret") {
		t.Fatalf("snapshot result missing or secret-bearing: %d %s", response.Code, response.Body)
	}
}

func TestConnectivityAdminHistoryRejectsInvalidFiltersAndBounds(t *testing.T) {
	handler, _, _, host := connectivityHTTPFixture(t)
	actor := model.User{ID: "admin", Role: "admin", TelegramID: 99}
	for _, query := range []string{
		"?hostUuid=bad", "?hostUuid=00000000-0000-0000-0000-000000000000", "?limit=0", "?limit=201", "?limit=no", "?cursor=garbage",
	} {
		response := connectivityHTTPRequest(handler, http.MethodGet, "/api/v1/admin/host-connectivity/history"+query, "", actor)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "CONNECTIVITY_INVALID_HISTORY_QUERY") {
			t.Fatalf("invalid query accepted: query=%s status=%d body=%s", query, response.Code, response.Body)
		}
	}
	response := connectivityHTTPRequest(handler, http.MethodGet, "/api/v1/admin/host-connectivity/history?hostUuid="+host+"&limit=1", "", actor)
	var page connectivity.HistoryPage
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("empty history response: %d %s", response.Code, response.Body)
	}
}
